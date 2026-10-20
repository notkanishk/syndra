"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { UNKNOWN_PROJECT } from "@/components/names/Unresolved";
import { Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { NextStep } from "@/components/ui/NextStep";
import { WouldCancelQueued } from "@/lib/api-client";
import { armAck, type QueuedChange } from "@/lib/cancel-confirm";
import { formatAgoShort, humanizeKey } from "@/lib/format";
import { useNameResolver } from "@/lib/queries/useNameResolver";

/**
 * Undoing a change that's still waiting (A-04 §14, proposed/CancelQueuedStep.md).
 *
 * Never invisibly (owner, 15 Oct): a write that would cancel changes waiting for
 * you to send is refused by the backend until the operator has seen exactly
 * those changes. `request()` throws the refusal to whoever made the write, and
 * that dialog shows it as a step pushed inside its own sheet: sheets never
 * stack, and no route can skip it because the backend refuses until told.
 *
 *   const step = useCancelStep({ host: "Give Rohan a role", goAhead: "give Member" });
 *   const done = await step.run(() => grant.mutateAsync(input));   // KEPT, or the write's result
 *   <PushedStep step={step}>…the form…</PushedStep>
 *
 * A write with no sheet of its own (Approve on a queue row) passes
 * `standalone` and renders `step.element`, a small sheet holding only the step.
 */

/** `run` resolves with this when the operator keeps the waiting changes. */
export const KEPT = Symbol("kept");

export interface CancelStepOptions {
  /** The host's name, shown as the header chip: "Give Rohan Mehta a role". */
  host: string;
  /** What goes ahead, finishing "Cancel it and …": "give Member", "remove", "approve". */
  goAhead: string;
  /** The host's own tone: accent, or danger for a removal. */
  tone?: "accent" | "danger";
  /** No sheet of its own: `element` is a small sheet holding only the step. */
  standalone?: boolean;
}

interface Asking {
  changes: QueuedChange[];
  fresh: Set<string>;
  /** Going ahead was chosen and the write is being sent again. */
  busy: boolean;
  /** Put away (kept or finished) but still drawn while the form fades back. */
  closed: boolean;
  answer: (goAhead: boolean) => void;
}

export interface CancelStep {
  run: <T>(write: () => Promise<T>) => Promise<T | typeof KEPT>;
  /** The step is on screen (or fading back to the form). */
  asking: Asking | null;
  open: boolean;
  /** Footer note after Keep: "Nothing changed. Sam's removal is still waiting." */
  note: ReactNode;
  /** The faint line after going ahead, under the host's result. */
  after: ReactNode;
  /** Standalone only: the small sheet. */
  element: ReactNode;
  view: ReactNode;
}

const TINT_MS = 120;
const CAP = 5;

function reducedMotion(): boolean {
  return typeof window === "undefined" || typeof window.matchMedia !== "function" || window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

const firstOf = (name: string) => name.trim().split(/\s+/)[0] || name;
const isRemoval = (change: QueuedChange) => change.op_type === "revoke";

function useWho() {
  const resolver = useNameResolver();
  return {
    person: (id: string) => resolver.resolveUser(id).value?.display_name ?? "this person",
    queuer: (change: QueuedChange) => {
      if (change.initiated_by_name) return change.initiated_by_name;
      if (!change.initiated_by || change.initiated_by === "system") return "Syndra";
      return resolver.resolveUser(change.initiated_by).value?.display_name ?? "an account Syndra can't name";
    },
    place: (change: QueuedChange) => resolver.resolveProject(change.project_id).value?.name || UNKNOWN_PROJECT,
    role: (change: QueuedChange) =>
      change.role_keys
        .map((key) => resolver.resolveRole(change.project_id, key).value?.display_name ?? humanizeKey(key))
        .join(", "),
  };
}
type Who = ReturnType<typeof useWho>;

interface Group {
  key: string;
  fresh: boolean;
  by: string;
  when: string;
  reason: string;
  changes: QueuedChange[];
}

/** Who queued it, the same minute, the same reason: one group, one who-line, one quote. New ones first. */
function group(changes: QueuedChange[], fresh: Set<string>, who: Who): Group[] {
  const groups: Group[] = [];
  for (const change of changes) {
    const isNew = fresh.has(change.id);
    const by = who.queuer(change);
    const key = `${isNew}|${change.initiated_by || by}|${change.queued_at.slice(0, 16)}|${change.reason ?? ""}`;
    const found = groups.find((g) => g.key === key);
    if (found) found.changes.push(change);
    else groups.push({ key, fresh: isNew, by, when: change.queued_at, reason: change.reason ?? "", changes: [change] });
  }
  return [...groups.filter((g) => g.fresh), ...groups.filter((g) => !g.fresh)];
}

function kindOf(changes: QueuedChange[], plural: boolean): string {
  const removal = changes.every(isRemoval);
  const give = changes.every((c) => !isRemoval(c));
  if (removal) return plural ? "removals" : "removal";
  if (give) return plural ? "grants" : "grant";
  return plural ? "changes" : "change";
}

function titleOf(changes: QueuedChange[]): string {
  return changes.length === 1
    ? `This cancels a ${kindOf(changes, false)} that's waiting`
    : `This cancels ${changes.length} ${kindOf(changes, true)} that are waiting`;
}

function ledeOf(changes: QueuedChange[], who: Who): string {
  const people = new Set(changes.map((c) => c.user_id));
  const removal = changes.every(isRemoval);
  const give = changes.every((c) => !isRemoval(c));
  if (people.size !== 1 || (!removal && !give)) return "None of it has reached Zitadel, so nobody's access has changed yet.";
  const first = firstOf(who.person(changes[0].user_id));
  if (changes.length === 1) {
    return removal
      ? `It hasn't reached Zitadel, so ${first} still has ${who.role(changes[0])} today.`
      : `It hasn't reached Zitadel, so ${first} doesn't have ${who.role(changes[0])} yet.`;
  }
  return removal
    ? `None of it has reached Zitadel, so ${first} still has all of it today.`
    : `None of it has reached Zitadel, so ${first} doesn't have any of it yet.`;
}

/** One queuer for all of it, as a first name; otherwise null. */
function soleQueuer(changes: QueuedChange[], who: Who): string | null {
  const names = new Set(changes.map((c) => firstOf(who.queuer(c))));
  return names.size === 1 ? Array.from(names)[0] : null;
}

function keptNote(changes: QueuedChange[], who: Who): string {
  const by = soleQueuer(changes, who);
  if (changes.length === 1) return `Nothing changed. ${by}'s ${kindOf(changes, false)} is still waiting.`;
  const count = `${changes.length} ${kindOf(changes, true)}`;
  return by ? `Nothing changed. ${by}'s ${count} are still waiting.` : `Nothing changed. The ${count} are still waiting.`;
}

function afterLine(changes: QueuedChange[], who: Who): string {
  const by = soleQueuer(changes, who);
  const when = formatAgoShort(changes[0].queued_at);
  const kind = kindOf(changes, changes.length !== 1);
  const subject =
    changes.length === 1
      ? `The ${kind} ${by} queued ${when} is`
      : by
        ? `The ${changes.length} ${kind} ${by} queued ${when} are`
        : `The ${changes.length} ${kind} that were waiting for you to send are`;
  return `${subject} cancelled, so nothing goes to Zitadel. Change history records ${changes.length === 1 ? "both" : "them all"}.`;
}

function freshLine(groups: Group[]): string {
  const fresh = groups.filter((g) => g.fresh);
  const n = fresh.reduce((sum, g) => sum + g.changes.length, 0);
  const bys = new Set(fresh.map((g) => firstOf(g.by)));
  const latest = fresh.map((g) => g.when).sort().at(-1) ?? "";
  const when = formatAgoShort(latest);
  const more = n === 1 ? "One more" : `${n} more`;
  const them = n === 1 ? "it" : "them";
  const was = n === 1 ? "it's" : "they're";
  const by = bys.size === 1 ? `${Array.from(bys)[0]} queued ${them}` : `They were queued`;
  return `${more} since you looked. ${by} ${when}, so ${was} added at the top and nothing was changed.`;
}

export function useCancelStep(options: CancelStepOptions): CancelStep {
  const [asking, setAsking] = useState<Asking | null>(null);
  const [kept, setKept] = useState<QueuedChange[] | null>(null);
  const [cancelled, setCancelled] = useState<QueuedChange[] | null>(null);
  const who = useWho();
  const returnFocus = useRef<HTMLElement | null>(null);

  const run = useCallback(async function run<T>(write: () => Promise<T>): Promise<T | typeof KEPT> {
    setKept(null);
    setCancelled(null);
    let shown: QueuedChange[] = [];
    for (;;) {
      try {
        const result = await write();
        if (shown.length) setCancelled(shown);
        returnFocus.current = null;
        setAsking((a) => (a ? { ...a, closed: true, busy: false } : a));
        return result;
      } catch (error) {
        if (!(error instanceof WouldCancelQueued)) {
          returnFocus.current = null;
          setAsking((a) => (a ? { ...a, closed: true, busy: false } : a));
          throw error;
        }
        const before = new Set(shown.map((c) => c.id));
        const seen = new Set<string>();
        const changes = error.cancels.filter((c) => !seen.has(c.id) && seen.add(c.id));
        const fresh = new Set(changes.filter((c) => c.new || (before.size > 0 && !before.has(c.id))).map((c) => c.id));
        if (!returnFocus.current) returnFocus.current = document.activeElement as HTMLElement | null;
        const goAhead = await new Promise<boolean>((answer) =>
          setAsking({ changes, fresh, busy: false, closed: false, answer }),
        );
        if (!goAhead) {
          setAsking((a) => (a ? { ...a, closed: true } : a));
          setKept(changes);
          const back = returnFocus.current;
          returnFocus.current = null;
          setTimeout(() => back?.isConnected && back.focus(), TINT_MS + 20);
          return KEPT;
        }
        shown = changes;
        armAck(error.method, error.path, changes.map((c) => c.id));
        setAsking((a) => (a ? { ...a, busy: true } : a));
      }
    }
  }, []);

  const open = asking !== null && !asking.closed;

  // Escape means keep them, whichever sheet it would otherwise close.
  useEffect(() => {
    if (!open || asking?.busy) return;
    function keep(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopImmediatePropagation();
      asking?.answer(false);
    }
    window.addEventListener("keydown", keep, true);
    return () => window.removeEventListener("keydown", keep, true);
  }, [open, asking]);

  const view = asking ? <StepBody asking={asking} options={options} who={who} /> : null;

  return {
    run,
    asking,
    open,
    note: kept ? (
      <span className="tint-in flex flex-col gap-1">
        <span>{keptNote(kept, who)}</span>
        <NextStep />
      </span>
    ) : null,
    after: cancelled ? (
      <p className="mx-6 mb-1 text-[13px] leading-[1.5] text-faint">{afterLine(cancelled, who)}</p>
    ) : null,
    element:
      options.standalone && asking && !asking.closed ? (
        <Modal open onClose={() => asking.answer(false)} busy={asking.busy} size="sm" labelledBy="cancel-queued-title">
          {view}
        </Modal>
      ) : null,
    view,
  };
}

function StepBody({ asking, options, who }: { asking: Asking; options: CancelStepOptions; who: Who }) {
  const { changes, fresh, busy } = asking;
  const [showAll, setShowAll] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const groups = group(changes, fresh, who);
  const capped = !showAll && changes.length > CAP;
  let room = capped ? CAP : Infinity;
  const visible = groups
    .map((g) => {
      const take = g.changes.slice(0, Math.max(0, room));
      room -= take.length;
      return { ...g, changes: take };
    })
    .filter((g) => g.changes.length > 0);
  const noReason = changes.some((c) => !c.reason);
  const askWho = noReason ? firstOf(who.queuer(changes.find((c) => !c.reason)!)) : "";
  const n = changes.length;
  const confirmLabel = n === 1 ? `Cancel it and ${options.goAhead}` : `Cancel all ${n} and ${options.goAhead}`;

  useEffect(() => {
    root.current?.querySelector<HTMLElement>("[data-keep]")?.focus();
  }, []);

  return (
    <div ref={root}>
      <ModalHeader
        chip={<span className="type-label">{options.host}</span>}
        title={titleOf(changes)}
        titleId="cancel-queued-title"
        lede={ledeOf(changes, who)}
      />
      {fresh.size > 0 && (
        <p className="mx-6 -mt-1 mb-3 flex items-start gap-2 text-[13.5px] leading-[1.5] text-ink">
          <StatusDot tone="accent" className="mt-[7px]" />
          {freshLine(groups)}
        </p>
      )}
      <div className="mx-6 overflow-hidden rounded-inner border border-line-strong">
        {visible.map((g, index) => (
          <section
            key={g.key}
            className={`flex flex-col gap-1 px-4 py-3 text-[14px] ${index > 0 ? "border-t border-line" : ""} ${
              g.fresh ? "flash bg-accent-soft" : ""
            }`}
          >
            {g.fresh && <span className="type-label text-accent-text">New since you looked</span>}
            {g.changes.map((change) => (
              <p key={change.id}>
                <strong className="font-semibold">{isRemoval(change) ? "Removing" : "Giving"}</strong>{" "}
                {who.place(change)} · <Mono>{who.role(change)}</Mono> {isRemoval(change) ? "from" : "to"}{" "}
                {who.person(change.user_id)}
              </p>
            ))}
            <p className="text-[13px] text-muted">
              Queued by {g.by} · {formatAgoShort(g.when)}
            </p>
            {g.reason ? (
              <p className="text-ink">“{g.reason}”</p>
            ) : (
              <p className="text-[13px] text-faint">{firstOf(g.by)} gave no reason.</p>
            )}
          </section>
        ))}
        {capped && (
          <p className="flex items-center gap-2 border-t border-line px-4 py-2.5 text-[13px] text-muted">
            Showing {CAP} of {n} <span aria-hidden>·</span>
            <button
              type="button"
              onClick={() => setShowAll(true)}
              className="font-semibold text-accent-text motion-tint hover:brightness-110"
            >
              Show all {n}
            </button>
          </p>
        )}
      </div>
      <ModalFooter
        note={
          <>
            Nothing changes until you choose.
            {noReason && ` If you're unsure why ${askWho} queued it, keep it and ask ${askWho}.`}
          </>
        }
      >
        <Button data-keep variant="outline" disabled={busy} onClick={() => asking.answer(false)}>
          Keep it
        </Button>
        <Button
          variant={options.tone === "danger" ? "dangerConfirm" : "accent"}
          isPending={busy}
          onClick={() => asking.answer(true)}
        >
          {confirmLabel}
        </Button>
      </ModalFooter>
    </div>
  );
}

/**
 * The host sheet's content, with the step pushed over it (S2). The form fades
 * first, then the sheet's height settles to the step; the sheet itself doesn't
 * move. Keep runs the reverse and the form is as it was, because it stays
 * mounted behind the step. Reduced motion: each appears in place.
 */
export function PushedStep({ step, children }: { step: CancelStep; children: ReactNode }) {
  const wanted = step.open ? "step" : "form";
  const [shown, setShown] = useState<"form" | "step">("form");
  const [fading, setFading] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const from = useRef(0);

  useEffect(() => {
    if (wanted === shown) return;
    if (reducedMotion()) {
      setShown(wanted);
      return;
    }
    from.current = box.current?.offsetHeight ?? 0;
    setFading(true);
    const timer = setTimeout(() => {
      setShown(wanted);
      setFading(false);
    }, TINT_MS);
    return () => clearTimeout(timer);
  }, [wanted, shown]);

  useLayoutEffect(() => {
    const el = box.current;
    const start = from.current;
    from.current = 0;
    if (!el || !start) return;
    const end = el.scrollHeight;
    if (start === end) return;
    el.style.overflow = "hidden";
    el.style.height = `${start}px`;
    void el.offsetHeight;
    el.style.transition = "height var(--duration-settle) var(--ease-settle)";
    el.style.height = `${end}px`;
    const done = () => {
      el.style.overflow = "";
      el.style.height = "";
      el.style.transition = "";
    };
    el.addEventListener("transitionend", done, { once: true });
    const fallback = setTimeout(done, 600);
    return () => clearTimeout(fallback);
  }, [shown]);

  return (
    <div ref={box}>
      <div data-step-form hidden={shown !== "form"} className={`motion-tint ${fading && shown === "form" ? "opacity-0" : ""}`}>
        {children}
      </div>
      {shown === "step" && step.view && (
        <div className={`${fading ? "opacity-0 motion-tint" : "tint-in"}`}>{step.view}</div>
      )}
    </div>
  );
}

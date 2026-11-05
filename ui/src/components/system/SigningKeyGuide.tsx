"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { ActionForecast, ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { CommandBlock } from "@/components/ui/CommandBlock";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { Modal, ModalFooter, mountedWhileOpen } from "@/components/ui/Modal";
import { request } from "@/lib/api-client";
import { formatShortDate } from "@/lib/format";
import { Unfold } from "@/lib/motion";
import { invalidateAfter } from "@/lib/queries/invalidation";
import { FORECAST, outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";

/** What the guide reads from GET /zitadel/action-rotation-status. */
export interface GuideStatus {
  key_installed?: boolean;
  last_rotated_at?: string | null;
  rotate_command?: string;
  register_command?: string;
}

/** POST /zitadel/action-key/rotate's answer. */
export interface RotateResult {
  /** partial: one key replaced, not the other; unknown: the answer was lost and the read-back failed (W4). */
  outcome: "verified" | "unverified" | "partial" | "unknown" | "failed";
  replaced_at?: string;
  replaced?: string[];
  probe?: { pass: boolean; class?: string; detail?: string };
  next_due_at?: string;
  /** A plain sentence, never an HTTP status: it arrives in a 200 body, past the error envelope. */
  reason?: string;
}

/** POST /zitadel/action-key/verify's answer. */
interface VerifyResult {
  probe: { pass: boolean; class?: string; detail?: string; at?: string };
  last_rotated_at?: string;
  next_due_at?: string;
}

type Result = { outcome: Outcome; word: string; final: boolean };

/** What the server said a replacement did, in the sheet's own words. */
export function replaceResult(r: RotateResult): { outcome: Outcome; word: string } {
  if (r.outcome === "verified") {
    return { outcome: { kind: "applied", message: `Replaced. A test sign-in got its claims.${nextDue(r.next_due_at)}` }, word: "Replaced" };
  }
  // ponytail: "no_change" is the neutral seat: no bloom, and no "Nothing was
  // changed" (a key WAS, or may have been, replaced). OutcomeKind has no "done, unconfirmed".
  if (r.outcome === "unverified" && r.probe?.class === "not_set_up") {
    return {
      outcome: {
        kind: "no_change",
        message: "Replaced. No test sign-in could run, because SYNDRA_EXTERNAL_URL is not set",
        detail: "Syndra accepts the old key and the new one for 7 days.",
        next: "Set SYNDRA_EXTERNAL_URL on the Syndra server and restart it, so a test sign-in can confirm the new key.",
      },
      word: "Replaced, not confirmed",
    };
  }
  if (r.outcome === "unverified") {
    return {
      outcome: {
        kind: "no_change",
        message: `Replaced, but the test sign-in after it didn't get its claims. ${UNCONFIRMED_HOLD}`,
        detail: "Claims delivery, above, says what the test sign-in got.",
        next: "Open the Claims delivery section on this page and fix what it names.",
      },
      word: "Replaced, not confirmed",
    };
  }
  if (r.outcome === "partial") {
    return {
      outcome: {
        kind: "no_change",
        message: `${r.reason ?? "Zitadel replaced one key but not the other."} Syndra uses the new key, and the old one still works for the other`,
        next: "Press Replace now again to finish, or use Do it by hand on the Action signing key row.",
      },
      word: "Partly replaced",
    };
  }
  if (r.outcome === "unknown") {
    return {
      outcome: {
        kind: "no_change",
        message: r.reason ?? "Zitadel didn't answer, and Syndra couldn't read the key back, so it can't tell whether it was replaced",
        detail: "The old key keeps working either way.",
        next: "Check the Action signing key row in a minute. If it still says Not known, press Replace now.",
      },
      word: "Not known",
    };
  }
  return {
    outcome: { kind: "failed", message: `Zitadel didn't replace the key.${r.reason ? ` ${r.reason}` : ""} The old key still works` },
    word: "Not replaced",
  };
}

/** W11: what an unconfirmed replacement keeps, and how it settles. */
export const UNCONFIRMED_HOLD =
  "Syndra accepts the old key and the new one for 7 days. It marks the new key confirmed once a regular test sign-in passes, and won't replace it again before it is next due.";

function nextDue(iso?: string): string {
  return iso ? ` Next due ${formatShortDate(iso)}.` : "";
}

/** A key Syndra saw arrive, checked by a test sign-in. */
function verifyResult(r: VerifyResult, setup: boolean): { outcome: Outcome; word: string } {
  if (r.probe.pass) {
    return {
      outcome: { kind: "applied", message: `${setup ? "Set up" : "Replaced"}. A test sign-in got its claims.${nextDue(r.next_due_at)}` },
      word: setup ? "Set up" : "Replaced",
    };
  }
  return {
    // The key did change; only the check failed. Neutral, never "Nothing was changed".
    outcome: {
      kind: "no_change",
      message: `Syndra has the new key, but the test sign-in didn't get its claims${r.probe.detail ? `: ${r.probe.detail}` : ""}.`,
      detail: "Claims delivery, above, says what to check. The old key keeps working for 15 minutes.",
      next: "Open the Claims delivery section on this page and fix what it names.",
    },
    word: "Not confirmed",
  };
}

/** How long the guide waits for the new key before offering the long way round. */
export const NOT_SEEN_AFTER_MS = 3 * 60_000;
/** How often step 1 asks Syndra to look for the new key. */
export const WATCH_EVERY_MS = 3_000;

// Closing mid-way and coming back returns to the same step, with the same "before".
const KEPT = "syndra:signing-key-guide";
const KEPT_FOR_MS = 60 * 60_000;
type Kept = { mode: "replace" | "setup"; step: 1 | 2; before: string | null; since: number };

function recall(mode: Kept["mode"]): Kept | null {
  try {
    const kept = JSON.parse(window.sessionStorage.getItem(KEPT) ?? "null") as Kept | null;
    return kept && kept.mode === mode && Date.now() - kept.since < KEPT_FOR_MS ? kept : null;
  } catch {
    return null;
  }
}

function keep(value: Kept | null) {
  try {
    if (value) window.sessionStorage.setItem(KEPT, JSON.stringify(value));
    else window.sessionStorage.removeItem(KEPT);
  } catch {
    // A blocked store is a guide that starts again; nothing else.
  }
}

/** The key Syndra holds is not the one it held when the guide opened. */
function arrived(s: GuideStatus | undefined, kept: Kept): boolean {
  if (!s) return false;
  if (kept.mode === "setup") return Boolean(s.key_installed || s.last_rotated_at);
  return Boolean(s.last_rotated_at) && s.last_rotated_at !== kept.before;
}

/**
 * Do it by hand (route-zitadel §2, m=guide): one thing per step, the command in
 * a copy row, and the last step reads the key again so the dialog becomes its
 * result. Syndra watches for the new key, so there is no "Next" to press, no
 * settings to type and no restart: both keys are followed from Zitadel and
 * .env is only where Syndra starts. The long way round waits behind "Not seen
 * yet?" and only appears once the key is late.
 *
 * The only thing here that changes anything is "Let Syndra do it now", the same
 * act as Replace now. Everything else watches and checks.
 */
export const SigningKeyGuide = mountedWhileOpen(function SigningKeyGuide({
  status,
  onClose,
}: {
  open: boolean;
  status?: GuideStatus;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [kept, setKept] = useState<Kept>(() => {
    const mode: Kept["mode"] = status?.key_installed ? "replace" : "setup";
    return recall(mode) ?? { mode, step: 1, before: status?.last_rotated_at ?? null, since: Date.now() };
  });
  // Fixed for this opening: a key that arrives mid-way does not turn "set up" into "replace".
  const setup = kept.mode === "setup";
  const [result, setResult] = useState<Result | null>(null);
  const [late, setLate] = useState(false);
  const [longWay, setLongWay] = useState(false);
  useEffect(() => keep(kept), [kept]);

  const watch = useQuery({
    queryKey: ["zitadel", "rotation", "watch"],
    queryFn: () => request<GuideStatus>("/zitadel/action-rotation-status?watch=1"),
    enabled: kept.step === 1,
    refetchInterval: WATCH_EVERY_MS,
    retry: false,
  });

  const settle = () => invalidateAfter(client, "signingKey");
  const verify = useMutation({
    mutationFn: () => request<VerifyResult>("/zitadel/action-key/verify", { method: "POST", body: {} }),
    onSettled: settle,
  });
  const rotate = useMutation({
    mutationFn: () => request<RotateResult>("/zitadel/action-key/rotate", { method: "POST", body: { force: true } }),
    onSettled: settle,
  });

  const check = async () => {
    setResult(null);
    try {
      setResult({ ...verifyResult(await verify.mutateAsync(), setup), final: true });
    } catch (err) {
      setResult({ outcome: outcomeFromError(err), word: "Not checked", final: false });
    }
  };

  // The new key arrived (or had, while the guide was closed): on to the check, with no click.
  const started = useRef(false);
  const seen = kept.step === 1 ? arrived(watch.data, kept) : kept.step === 2;
  useEffect(() => {
    if (!seen || started.current) return;
    started.current = true;
    setKept((k) => ({ ...k, step: 2 }));
    void check();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once per opening
  }, [seen]);

  useEffect(() => {
    if (kept.step !== 1) return;
    const t = setTimeout(() => setLate(true), Math.max(0, kept.since + NOT_SEEN_AFTER_MS - Date.now()));
    return () => clearTimeout(t);
  }, [kept.step, kept.since]);

  const letSyndra = async () => {
    started.current = true;
    setKept((k) => ({ ...k, step: 2 }));
    setResult(null);
    try {
      setResult({ ...replaceResult(await rotate.mutateAsync()), final: true });
    } catch (err) {
      setResult({ outcome: outcomeFromError(err), word: "Not replaced", final: false });
    }
  };

  // Cancel takes the focus, not the command's Copy above it. This runs after
  // the Modal's own trap (a child's effect runs first), as CancelQueuedStep does.
  const head = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const buttons = head.current?.closest("[role=dialog]")?.querySelectorAll<HTMLButtonElement>("button") ?? [];
    Array.from(buttons).find((b) => b.textContent === "Cancel")?.focus();
  }, []);

  const busy = verify.isPending || rotate.isPending;
  const done = result?.final ?? false;
  const close = () => {
    if (done) keep(null); // finished: the next opening starts clean
    onClose();
  };
  const command = setup
    ? (status?.register_command ?? "make zitadel-actions-register")
    : (status?.rotate_command ?? "make zitadel-actions-rotate-key");

  return (
    <Modal open onClose={close} busy={busy} size="md" labelledBy="key-guide-title">
      <div ref={head} className="flex flex-col gap-3 px-6 pt-[22px]">
        <div className="flex items-center gap-3">
          <span className="text-[12.5px] font-semibold text-muted">Step {kept.step} of 2</span>
          <span aria-hidden className="flex flex-1 gap-1">
            {[1, 2].map((i) => (
              <span key={i} className={`h-1 flex-1 rounded-pill ${i <= kept.step ? "bg-accent" : "bg-line-strong"}`} />
            ))}
          </span>
        </div>
        <h2 id="key-guide-title" className="type-dialog-title">
          {kept.step === 1 ? (setup ? "Register Syndra with Zitadel" : "Make a new key") : "Check the new key"}
        </h2>
        <p className="text-[14.5px] leading-[1.55] text-muted">
          {kept.step === 1
            ? "Run this on the Syndra server."
            : "Syndra signs a test sign-in with the new key and checks it gets its claims."}
        </p>
      </div>

      <div className="flex flex-col gap-3 px-6 pb-1 pt-4">
        {kept.step === 1 && (
          <>
            <CommandBlock command={command} />
            <p role="status" className="flex items-center gap-2 text-[13.5px] text-muted">
              <StatusDot tone="warn" className="breathe" />
              Watching for the new key. This moves on by itself once Syndra sees it.
            </p>
            {late && (
              <NotSeenYet open={longWay} onToggle={() => setLongWay((v) => !v)} setup={setup} />
            )}
          </>
        )}
        {kept.step === 2 && !result && busy &&
          (setup ? (
            <p role="status" className="flex items-center gap-2 text-[13.5px] font-semibold text-ink">
              <StatusDot tone="warn" className="breathe" />
              Checking the new key
            </p>
          ) : (
            <ActionForecast word={FORECAST.replace} />
          ))}
        {result && <ActionOutcome outcome={result.outcome} word={result.word} />}
      </div>

      <ModalFooter
        cancel={done ? undefined : { onClick: close, disabled: busy }}
        note={
          kept.step === 1 && !setup
            ? "Or let Syndra make the new key itself: it starts using it at once, and sign-ins carry on."
            : undefined
        }
      >
        {done ? (
          <Button variant="outline" onClick={close}>
            Close
          </Button>
        ) : kept.step === 1 ? (
          !setup && (
            <Button variant="accent" isPending={rotate.isPending} onClick={letSyndra}>
              Let Syndra do it now
            </Button>
          )
        ) : (
          result && (
            <Button variant="accent" isPending={busy} onClick={check}>
              Check again
            </Button>
          )
        )}
      </ModalFooter>
    </Modal>
  );
});

/** The long way round, folded, for when the new key is late. Never shown upfront. */
function NotSeenYet({ open, onToggle, setup }: { open: boolean; onToggle: () => void; setup: boolean }) {
  return (
    <div className="flex flex-col">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex min-h-[var(--tap)] items-center gap-2 text-left text-[13.5px] font-semibold text-muted"
      >
        <FoldChevron open={open} />
        Not seen yet?
      </button>
      <Unfold open={open}>
        <div className="flex flex-col gap-3 pb-2 text-[13.5px] leading-[1.55] text-muted">
          <p>
            Syndra reads the key from Zitadel every minute, so a new one usually shows within a minute of the
            command finishing. Check the command ended without an error, and that it ran against the Zitadel
            this Syndra uses.
          </p>
          <p>
            Only if the keyring line on this page says Syndra isn&rsquo;t allowed to read the key from
            Zitadel: set these to what the command printed, then restart the backend.
          </p>
          <CopyableValue label="Claims key setting" value="ZITADEL_ACTION_SIGNING_KEY" />
          {!setup && <CopyableValue label="Event key setting" value="ZITADEL_EVENT_SIGNING_KEY" />}
          <CommandBlock command="docker compose restart backend" />
        </div>
      </Unfold>
    </div>
  );
}

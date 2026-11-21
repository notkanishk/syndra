"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";

import { SyndraMark } from "@/components/shell/SyndraMark";
import { GO_TO_ASKS, QUESTION_START, useGoToAsk } from "@/components/ui/goToAsk";
import { Kbd } from "@/components/ui/Kbd";
import { Modal } from "@/components/ui/Modal";
import { GO_TO_SCOPES, goToNamed, goToRows, offeredScope, recallLately, sigilScope, type GoToRow, type GoToScope } from "@/lib/go-to";
import { ARRIVE_MS, FLASH_MS, SETTLE_MS, TINT_MS, stillMotion } from "@/lib/motion";
import type { NavEntry } from "@/lib/nav";
import { useIndicatorsRead } from "@/lib/queries/useIndicators";
import { useNameCatalog } from "@/lib/queries/useNameCatalog";
import { useUiView } from "@/lib/ui-view";
import { useDebounce } from "@/lib/useDebounce";
import { useIsPhone } from "@/lib/useViewport";

/**
 * Go to: one way to reach anything in Syndra by typing its name (proposed/GoTo.md, A-19 P4), as chosen
 * (A-26 §0). It opens pages and follows answers; it never runs a verb. It is opened by a key dozens of
 * times a day, so it appears at once and leaves at once (A-24 §8): a still Modal, pinned at the top at
 * every width, a phone's too, and the phone's sheet opens this same one. Its name is the dialog's
 * `aria-label`, since no title shows it.
 */
type GoToProps = {
  open: boolean;
  onClose: () => void;
  /** Which pages may be listed: the current view's nav tree. */
  nav: NavEntry[];
};

/** What the box is for, by who is asking (A-26 §0): a member has pages only; Advanced may also ask. */
export const GO_TO_PLACEHOLDER = {
  member: "A page",
  basic: "A person, a role, or a page",
  advanced: "A person, a role, a page, or a question",
} as const;

/** A phone's field says less: the board's words (A-26 §0). */
export const GO_TO_PHONE_PLACEHOLDER = { member: "A page", basic: "A name, or a page", advanced: "A name, or a question" } as const;

export function GoTo({ open, onClose, nav }: GoToProps) {
  return (
    <Modal open={open} onClose={onClose} size="sm" still pinned glass label="Go to">
      <GoToPane nav={nav} onGo={onClose} />
    </Modal>
  );
}

/** What leaving a scope by its pill says, and what the pill is (A-26 §0, item 6). */
export const scopeLabel = (scope: GoToScope) =>
  `Looking only among ${GO_TO_SCOPES[scope].word.toLowerCase()}. Look everywhere again.`;

/** The light behind Go to at its largest: as wide as a wide window, a little over half a laptop's height. */
const GO_TO_LIGHT = { width: 1180, height: 620 };

/**
 * Where the light behind Go to sits, in the window (A-26, the lamp's light; A-25 §7): on the lamp, unless
 * the lamp is too near an edge for a wide pool (a phone), then the window's middle; never more than twice
 * its centre's distance from the nearest edge, so its closest-side pool is spent before every edge.
 */
export function goToLightBox(lamp: { x: number; y: number }, view: { width: number; height: number }) {
  const x = Math.min(lamp.x, view.width - lamp.x) < 280 ? view.width / 2 : lamp.x;
  const width = Math.max(0, Math.min(GO_TO_LIGHT.width, 2 * Math.min(x, view.width - x)));
  const height = Math.max(0, Math.min(GO_TO_LIGHT.height, 2 * Math.min(lamp.y, view.height - lamp.y)));
  return { x, y: lamp.y, width, height };
}

/**
 * How far the lamp's halo flares, as a scale of its resting size. Capped so the flared halo is still spent
 * inside the field (A-25 rule 1: no box ends a light): its resting radius is 0.92 of the mark, and the
 * field's half-height is the room it has (34px over a 32px mark; 31px over 28px on a phone).
 */
export const GO_TO_FLARE = 1.15;

type Thing = { type: "answer"; key: "answer" } | { type: "ask"; key: string; text: string } | ({ type: "row" } & GoToRow);

/**
 * Go to as chosen (A-26 §0; owner, 21 Nov 2026). One glass pane: the lamp, the field, and one quiet line
 * at rest (the last place). Typing grows one list beneath, best match first. A kind's word offers its
 * scope ("Tab  only people"); Tab, the offer, or `@` `#` `/` first enters it, and Backspace on an empty
 * field gives the typing back. Esc clears everything typed or chosen, then closes. In Advanced a question
 * is answered in place, from Insights' own reads, once the typing has rested, and `↵` or `⌘↵` follows it
 * on the map. Entering a scope is one gesture that starts at the lamp; nothing about the field or the
 * typing ever waits for it.
 */
export function GoToPane({ nav, onGo }: { nav: NavEntry[]; onGo: () => void }) {
  const { audience, isOperator } = useUiView();
  const { catalog, bundles } = useNameCatalog();
  const { data: indicators } = useIndicatorsRead(isOperator);
  const here = usePathname();
  const phone = useIsPhone();
  const [lately] = useState(recallLately);
  const [query, setQuery] = useState("");
  const [scope, setScope] = useState<GoToScope | null>(null);
  // What was typed to make the scope ("peo"), given back by Backspace on an empty field.
  const [madeFrom, setMadeFrom] = useState("");
  // A scope was just left: said once to a screen reader, until the next thing is.
  const [left, setLeft] = useState(false);
  const [more, setMore] = useState(false);
  const [active, setActive] = useState(0);
  const base = useId();
  const listId = `${base}-list`;
  const names = audience !== "member";

  // A question is Advanced's: its answer is followed on Insights' map, which only Advanced has (A-26 §0a).
  // It is asked once the typing has rested a settle, so no read goes out per key and the answer does not
  // flicker through every prefix; until then the last answer stays.
  const asking = audience === "advanced" && !scope && QUESTION_START.test(query.trim());
  const settled = useDebounce(asking ? query : "", SETTLE_MS);
  const ask = useGoToAsk(settled, catalog?.users);
  const answer = asking ? ask.answer?.panel : undefined;
  const working = asking && !answer && (settled !== query || ask.working);
  const failed = asking && !answer && !working && ask.failed;
  const rows = useMemo(() => {
    const source = {
      nav,
      names,
      bundles: audience === "advanced",
      people: catalog?.users,
      projects: catalog?.projects,
      apps: catalog?.applications,
      bundleList: bundles,
      lately,
      here,
      waiting: indicators?.pending_propagation,
    };
    // Under an answer, the places its question names; a question not yet answered lists none.
    if (asking) return answer ? goToNamed(source, ask.named) : [];
    return goToRows(source, query, scope, more);
  }, [asking, answer, ask.named, nav, names, audience, catalog, bundles, lately, here, indicators?.pending_propagation, query, scope, more]);
  const things: Thing[] = [
    ...(answer ? [{ type: "answer", key: "answer" } as const] : []),
    ...(asking && !answer && !working && !failed ? GO_TO_ASKS.map((text) => ({ type: "ask", key: `ask:${text}`, text }) as const) : []),
    ...rows.map((row) => ({ type: "row", ...row }) as const),
  ];
  const current = things.length ? things[Math.min(active, things.length - 1)] : undefined;
  const offer = names && !scope && !asking ? offeredScope(query) : null;
  const optionId = (thing: Thing) => `${base}-${thing.key.replace(/[^\w-]/g, "_")}`;
  const typed = query.trim() !== "";
  const nothing = typed && !asking && !things.length;
  const nothingSaid = `Nothing by that name${scope ? ` among ${GO_TO_SCOPES[scope].word.toLowerCase()}` : ""}.`;

  const root = useRef<HTMLDivElement | null>(null);
  const field = useRef<HTMLInputElement | null>(null);
  const list = useRef<HTMLDivElement | null>(null);
  const activeRef = useRef<HTMLElement | null>(null);
  const answerRef = useRef<HTMLAnchorElement | null>(null);
  const halo = useRef<HTMLSpanElement | null>(null);
  const beam = useRef<HTMLSpanElement | null>(null);
  const wave = useRef<HTMLSpanElement | null>(null);

  // The light behind the pane: placed on the lamp before the first paint, and again when the window changes.
  useLayoutEffect(() => {
    const scrim = root.current?.closest<HTMLElement>(".go-to-scrim");
    const lamp = root.current?.querySelector("[data-lamp]");
    if (!scrim || !lamp) return;
    const place = () => {
      const at = lamp.getBoundingClientRect();
      const view = scrim.getBoundingClientRect();
      const box = goToLightBox(
        { x: at.left + at.width / 2 - view.left, y: at.top + at.height / 2 - view.top },
        { width: scrim.clientWidth, height: scrim.clientHeight },
      );
      scrim.style.setProperty("--light-x", `${box.x}px`);
      scrim.style.setProperty("--light-y", `${box.y}px`);
      scrim.style.setProperty("--light-w", `${box.width}px`);
      scrim.style.setProperty("--light-h", `${box.height}px`);
    };
    place();
    addEventListener("resize", place);
    return () => removeEventListener("resize", place);
  }, []);

  // The scope is the lamp's colour: on the window, so the light behind turns with the lamp.
  useEffect(() => {
    const scrim = root.current?.closest<HTMLElement>(".go-to-scrim");
    if (!scrim) return;
    if (scope) scrim.dataset.scope = scope;
    else delete scrim.dataset.scope;
  }, [scope]);

  // The chosen row stays in sight, and the list softens at the end that has more.
  useEffect(() => {
    activeRef.current?.scrollIntoView?.({ block: "nearest" });
    soften(list.current);
  }, [current?.key, things.length]);

  // The lamp opens once as an answer lands (D's), a settle after it shows. Once a question.
  const landed = useRef("");
  useEffect(() => {
    if (!answer || ask.title === landed.current) return;
    const timer = setTimeout(() => {
      landed.current = ask.title;
      if (!stillMotion()) flare(halo.current);
    }, SETTLE_MS);
    return () => clearTimeout(timer);
  }, [answer, ask.title]);

  function enterScope(next: GoToScope, typed: string, rest = "") {
    setScope(next);
    setMadeFrom(sigilScope(typed) ? "" : typed);
    setQuery(rest);
    setLeft(false);
    setMore(false);
    setActive(0);
    // The caret stays where it was: a tap on the offer must not leave the focus on a control that is gone.
    field.current?.focus();
    // One gesture, and it starts at the lamp. Never on leaving.
    const pane = root.current?.closest<HTMLElement>(".go-to-pane") ?? null;
    if (!stillMotion()) entered({ pane, scrim: pane?.parentElement ?? null, halo: halo.current, beam: beam.current, wave: wave.current });
  }
  function leaveScope(giveBack: boolean) {
    setScope(null);
    setQuery(giveBack ? madeFrom : "");
    setMadeFrom("");
    setLeft(true);
    setActive(0);
    field.current?.focus();
  }
  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    // An input method choosing its characters owns every key until it is done (229: Safari's last one).
    if (event.nativeEvent.isComposing || event.keyCode === 229) return;
    const empty = event.currentTarget.value === "";
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      // The rest of Lately, asked for, and what needs you last.
      if (event.key === "ArrowDown" && empty && !scope && !more) {
        setMore(true);
        setActive(1);
        return;
      }
      if (!things.length) return;
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((index) => (Math.min(index, things.length - 1) + step + things.length) % things.length);
    } else if (event.key === "Enter" && current) {
      event.preventDefault();
      if (current.type === "ask") {
        setQuery(current.text);
        setActive(0);
      } else if (current.type === "answer" || (answer && (event.metaKey || event.ctrlKey))) {
        // The answer is followed by its own link, as a link is: ↵ on it, ⌘↵ from a row under it.
        answerRef.current?.click();
      } else activeRef.current?.click();
    } else if (event.key === "Tab" && !event.shiftKey && offer) {
      // Tab is taken only while a scope is offered; otherwise it is the browser's.
      event.preventDefault();
      enterScope(offer, query.trim());
    } else if (event.key === "Backspace" && empty && scope) {
      event.preventDefault();
      leaveScope(true);
    } else if (event.key === "Escape" && (!empty || scope || more)) {
      // Esc in two stages: what is typed or chosen first, then the dialog.
      event.preventDefault();
      setQuery("");
      setScope(null);
      setMadeFrom("");
      setLeft(Boolean(scope));
      setMore(false);
      setActive(0);
    }
  }

  const placeholder = scope ? GO_TO_SCOPES[scope].hint : phone ? GO_TO_PHONE_PLACEHOLDER[audience] : GO_TO_PLACEHOLDER[audience];
  // What a screen reader is told, in one place: where it is looking, and what came of the typing.
  const said = asking
    ? answer
      ? answer.title
      : working
        ? "Working it out."
        : failed
          ? "Couldn't load this answer."
          : ""
    : nothing
      ? nothingSaid
      : scope
        ? `Looking only among ${GO_TO_SCOPES[scope].word.toLowerCase()}.`
        : left
          ? "Looking everywhere again."
          : "";
  const rowClass = (thing: Thing) => `go-to-row${thing.key === current?.key ? "" : " hover-tint"}`;

  return (
    <div ref={root} className="contents">
      <span ref={beam} aria-hidden className="go-to-beam" />
      <span ref={wave} aria-hidden className="go-to-wave" />
      <div className="go-to-field">
        <span className="go-to-lamp" data-lamp>
          <span ref={halo} aria-hidden className="go-to-halo" />
          <SyndraMark size={phone ? 28 : 32} />
        </span>
        {scope && (
          // The scope, Arc's way: a filled pill, its kind in words; a press looks everywhere again. It
          // answers the hand as a filled button does (A-25 moment 2).
          <button type="button" className="go-to-scope rim rim-filled" aria-label={scopeLabel(scope)} onClick={() => leaveScope(false)}>
            <span aria-hidden className="go-to-scope-light" />
            {GO_TO_SCOPES[scope].word}
          </button>
        )}
        <input
          ref={field}
          type="search"
          className="go-to-input"
          autoFocus
          role="combobox"
          aria-label="Go to"
          aria-expanded={things.length > 0}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={current ? optionId(current) : undefined}
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          enterKeyHint="go"
          placeholder={placeholder}
          value={query}
          onChange={(event) => {
            const value = event.target.value;
            const sigil = names && !scope ? sigilScope(value) : null;
            // A scope's own key first becomes the scope and is not kept; what follows it (a paste of
            // "@ada") is the typing inside it.
            if (sigil) return enterScope(sigil, GO_TO_SCOPES[sigil].sigil, value.slice(1));
            setQuery(value);
            setActive(0);
            setMore(false);
          }}
          onKeyDown={onKeyDown}
        />
        {offer && (
          <button type="button" className="go-to-offer" data-kind={offer} onClick={() => enterScope(offer, query.trim())}>
            <span className="hidden tablet:inline-flex">
              <Kbd>Tab</Kbd>
            </span>
            only {GO_TO_SCOPES[offer].word.toLowerCase()}
          </button>
        )}
      </div>

      {(asking && !answer) || nothing ? (
        // What came of the typing, when it is not a list: beside the list, never in it (only options are).
        <div className="go-to-say">
          {nothing ? (
            <>
              <span className="font-display text-title text-ink">{nothingSaid}</span>
              <span className="text-small text-muted">
                {scope ? "Backspace looks everywhere." : "Try fewer letters, or the start of any word in the name."}
              </span>
            </>
          ) : working ? (
            <span className="font-display text-title text-ink">Working it out.</span>
          ) : failed ? (
            <>
              <span className="font-display text-title text-ink">Couldn&apos;t load this answer.</span>
              <span className="text-small text-muted">
                <button type="button" className="go-to-link" onClick={ask.retry}>
                  Try again
                </button>
                , or{" "}
                <Link href={ask.href ?? "/insights"} className="go-to-link" onClick={onGo}>
                  open Insights
                </Link>
                .
              </span>
            </>
          ) : (
            <>
              <span className="font-display text-title text-ink">That isn&apos;t a question Syndra can answer.</span>
              <span className="text-small text-muted">She answers only what she can read exactly, and she never guesses. These she can:</span>
            </>
          )}
        </div>
      ) : null}

      <div
        ref={list}
        id={listId}
        role="listbox"
        aria-label="Where to go"
        className="go-to-list"
        hidden={!things.length}
        onScroll={(event) => soften(event.currentTarget)}
      >
        {answer && ask.href && (
          <Link
            id={optionId({ type: "answer", key: "answer" })}
            ref={(el) => {
              answerRef.current = el;
              if (current?.type === "answer") activeRef.current = el;
            }}
            href={ask.href}
            role="option"
            aria-selected={current?.type === "answer"}
            tabIndex={-1}
            onClick={onGo}
            className={`${rowClass({ type: "answer", key: "answer" })} flex-col items-start! gap-1 py-2.5`}
          >
            <span className="font-display text-title text-ink">{answer.title}</span>
            <span className="text-small text-muted">{answer.eyebrow}</span>
            {answer.lines.slice(0, 2).map((line, i) => (
              <span key={i} className="text-small text-muted">
                {line.lead ? `${line.lead} ` : ""}
                {line.text}
              </span>
            ))}
            <span className="flex flex-wrap items-baseline gap-x-3.5 text-small">
              <span className="go-to-link">Follow it on the map →</span>
              <span className="text-fine text-faint">{current?.type === "answer" ? "↵" : "⌘↵"}</span>
            </span>
          </Link>
        )}
        {things.map((thing) =>
          thing.type === "answer" ? null : thing.type === "ask" ? (
            <div
              key={thing.key}
              id={optionId(thing)}
              ref={thing.key === current?.key ? (el) => void (activeRef.current = el) : undefined}
              role="option"
              aria-selected={thing.key === current?.key}
              tabIndex={-1}
              onClick={() => {
                setQuery(thing.text);
                setActive(0);
                field.current?.focus();
              }}
              className={`${rowClass(thing)} cursor-pointer`}
            >
              <span className="go-to-name">{thing.text}…</span>
              <span className="flex-1" />
              <span className="flex-none text-small text-faint">Ask</span>
            </div>
          ) : (
            <Link
              key={thing.key}
              id={optionId(thing)}
              ref={thing.key === current?.key ? (el) => void (activeRef.current = el) : undefined}
              href={thing.href}
              role="option"
              aria-selected={thing.key === current?.key}
              tabIndex={-1}
              onClick={onGo}
              className={rowClass(thing)}
            >
              <span className="go-to-name">{thing.name}</span>
              <span className="min-w-0 flex-1 truncate text-small text-faint">{thing.descriptor}</span>
              {/* In a scope the pill already says what every row is. */}
              {!scope && <span className="flex-none text-small text-faint">{thing.tag}</span>}
            </Link>
          ),
        )}
      </div>
      <p role="status" className="sr-only">
        {said}
      </p>
    </div>
  );
}

/** A list that scrolls never cuts a line at its edge: it softens into the glass where there is more. */
function soften(list: HTMLElement | null) {
  if (!list) return;
  list.style.setProperty("--fade-top", list.scrollTop > 2 ? "18px" : "0px");
  list.style.setProperty("--fade-end", list.scrollTop + list.clientHeight < list.scrollHeight - 2 ? "26px" : "0px");
}

/** A motion token's curve, read from the stylesheet, so every curve is the token's own. */
const ease = (name: "--ease-flash" | "--ease-settle" | "--ease-press") =>
  getComputedStyle(document.documentElement).getPropertyValue(name).trim() || "ease";

/** The lamp's halo flares once: D's, a flash long, and spent inside the field (`GO_TO_FLARE`). */
function flare(halo: HTMLElement | null) {
  halo?.animate?.([{ opacity: 0.72, scale: 1 }, { opacity: 1, scale: GO_TO_FLARE, offset: 0.25 }, { opacity: 0.72, scale: 1 }], {
    duration: FLASH_MS,
    easing: ease("--ease-flash"),
  });
}

/**
 * Entering a scope (A-26 §0, "lamp lights, bar answers"): the lamp takes the scope's colour (the tint, in
 * CSS) and flares; the light behind swells; its light runs once through the bar, in the glass and along the
 * rim, up in a tint, out in 420ms and gone by a flash; and the bar presses in 2% and comes back, about its
 * own centre (100ms in, 140 back). Only the light and the press move; the field and the typing never wait.
 */
export const GESTURE = { press: 0.98, pressIn: 100, pressBack: 140, run: ARRIVE_MS, bright: 300 } as const;

function entered({ pane, scrim, halo, beam, wave }: Record<"pane" | "scrim" | "halo" | "beam" | "wave", HTMLElement | null>) {
  const all = TINT_MS + FLASH_MS;
  const up = TINT_MS / all;
  const once = [{ opacity: 0 }, { opacity: 1, offset: up }, { opacity: 1, offset: GESTURE.bright / all, easing: ease("--ease-press") }, { opacity: 0 }];
  const run = { duration: GESTURE.run, easing: ease("--ease-flash"), fill: "forwards" as const };
  for (const el of [beam, wave]) for (const a of el?.getAnimations?.() ?? []) a.cancel();
  flare(halo);
  scrim?.animate?.([{ opacity: 0.6 }, { opacity: 0.85, offset: up, easing: ease("--ease-press") }, { opacity: 0.6 }], { duration: all, pseudoElement: "::before" });
  beam?.animate?.(once, all);
  beam?.animate?.({ scale: ["0.2 1", "1 1"] }, run);
  wave?.animate?.(once, all);
  if (wave && pane) wave.animate?.({ "--reach": ["60px", `${pane.offsetWidth}px`] }, run);
  const total = GESTURE.pressIn + GESTURE.pressBack;
  pane?.animate?.(
    [
      { transform: "none", easing: ease("--ease-press") },
      { transform: `scale(${GESTURE.press})`, offset: GESTURE.pressIn / total, easing: ease("--ease-settle") },
      { transform: "none" },
    ],
    total,
  );
}

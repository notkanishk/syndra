"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";

import { SyndraMark } from "@/components/shell/SyndraMark";
import { EmptyState } from "@/components/states";
import { GO_TO_ASKS, QUESTION_START, useGoToAsk } from "@/components/ui/goToAsk";
import { Input } from "@/components/ui/Input";
import { Kbd } from "@/components/ui/Kbd";
import { Modal } from "@/components/ui/Modal";
import {
  GO_TO_SCOPES,
  flatItems,
  goTo,
  goToNamed,
  goToRows,
  offeredScope,
  recallLately,
  sigilScope,
  type GoToItem,
  type GoToRow,
  type GoToScope,
} from "@/lib/go-to";
import { ARRIVE_MS, FLASH_MS, SETTLE_MS, TINT_MS, stillMotion } from "@/lib/motion";
import type { NavEntry } from "@/lib/nav";
import { useIndicatorsRead } from "@/lib/queries/useIndicators";
import { useNameCatalog } from "@/lib/queries/useNameCatalog";
import { useUiView } from "@/lib/ui-view";
import { useIsPhone } from "@/lib/useViewport";

/**
 * Go to: one way to reach anything in Syndra by typing its name
 * (proposed/GoTo.md, A-19 P4). Opens pages, never runs a verb; nothing is
 * pinned. With nothing typed it lists Lately, the last places opened on this
 * device, above Pages, or on the phone above the sheet's own list (A-24 §8, Q13);
 * sign-out forgets them. The word "Search" belongs to the per-page
 * search, so it is "Go to" here.
 *
 * It is opened by a key dozens of times a day, so it appears at once and leaves
 * at once (A-24 §8): a still Modal. Escape clears what is typed, then closes.
 * A place it opens does not arrive (Q14).
 *
 * As drawn (A-24 frame 22): the field row is the top of the panel, with no
 * heading over it and no box round it, and the panel is pinned at a fixed top
 * so it never jumps as the list grows and shrinks. Its name is the dialog's
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

/** What Esc does now, said beside its key: with words typed it clears them first. */
const GO_TO_ESC = { typed: "clear, then close", empty: "close" } as const;

export function GoTo({ open, onClose, nav }: GoToProps) {
  return (
    <Modal open={open} onClose={onClose} size="sm" still pinned glass label="Go to">
      <GoToPane nav={nav} onGo={onClose} />
    </Modal>
  );
}

/** A phone's field says less: the board's words (A-26 §0). */
export const GO_TO_PHONE_PLACEHOLDER = { member: "A page", basic: "A name, or a page", advanced: "A name, or a question" } as const;

/** What leaving a scope by its pill says, and what the pill is (A-26 §0, item 6). */
export const scopeLabel = (scope: GoToScope) =>
  `Looking only among ${GO_TO_SCOPES[scope].word.toLowerCase()}. Look everywhere again.`;

type Thing = { type: "answer"; key: "answer" } | { type: "ask"; key: string; text: string } | ({ type: "row" } & GoToRow);

/**
 * Go to as chosen (A-26 §0; owner, 21 Nov 2026). One glass pane: the lamp, the field, and one quiet line
 * at rest (the last place). Typing grows one list beneath, best match first. A kind's word offers its
 * scope ("Tab  only people"); Tab, the offer, or `@` `#` `/` first enters it, and Backspace on an empty
 * field gives the typing back. Esc clears everything typed or chosen, then closes. In Advanced a question
 * is answered in place, from Insights' own reads, and `↵` or `⌘↵` follows it on the map. Entering a scope is
 * one gesture that starts at the lamp; nothing about the field or the typing ever waits for it.
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
  const [more, setMore] = useState(false);
  const [active, setActive] = useState(0);
  const base = useId();
  const listId = `${base}-list`;
  const names = audience !== "member";

  // A question is Advanced's: its answer is followed on Insights' map, which only Advanced has (A-26 §0a).
  const asking = audience === "advanced" && !scope && QUESTION_START.test(query.trim());
  const ask = useGoToAsk(asking ? query : "", catalog?.users);
  const answered = asking && !!ask.answer;
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
    if (asking) return answered ? goToNamed(source, ask.named) : [];
    return goToRows(source, query, scope, more);
  }, [asking, answered, ask.named, nav, names, audience, catalog, bundles, lately, here, indicators?.pending_propagation, query, scope, more]);
  const things: Thing[] = [
    ...(answered ? [{ type: "answer", key: "answer" } as const] : []),
    ...(asking && !answered && !ask.reading && !ask.status ? GO_TO_ASKS.map((text) => ({ type: "ask", key: `ask:${text}`, text }) as const) : []),
    ...rows.map((row) => ({ type: "row", ...row }) as const),
  ];
  const current = things.length ? things[Math.min(active, things.length - 1)] : undefined;
  const offer = names && !scope && !asking ? offeredScope(query) : null;
  const optionId = (thing: Thing) => `${base}-${thing.key.replace(/[^\w-]/g, "_")}`;

  const root = useRef<HTMLDivElement | null>(null);
  const field = useRef<HTMLInputElement | null>(null);
  const list = useRef<HTMLDivElement | null>(null);
  const activeRef = useRef<HTMLElement | null>(null);
  const answerRef = useRef<HTMLAnchorElement | null>(null);
  const point = useRef<HTMLSpanElement | null>(null);
  const halo = useRef<HTMLSpanElement | null>(null);
  const beam = useRef<HTMLSpanElement | null>(null);
  const wave = useRef<HTMLSpanElement | null>(null);

  // The scope is the lamp's colour: on the window, so the light behind turns with the lamp.
  useEffect(() => {
    const scrim = root.current?.closest<HTMLElement>(".go-to-scrim");
    if (!scrim) return;
    if (scope) scrim.dataset.scope = scope;
    else delete scrim.dataset.scope;
  }, [scope]);

  // The chosen row's point of light: placed beside it, one element that never resizes. The list softens
  // at the end that has more.
  useLayoutEffect(() => {
    const row = activeRef.current;
    const dot = point.current;
    if (row && dot) dot.style.transform = `translateY(${row.offsetTop + (row.offsetHeight - dot.offsetHeight) / 2}px)`;
    row?.scrollIntoView?.({ block: "nearest" });
    soften(list.current);
  }, [current?.key, things.length]);

  // The lamp opens once as an answer lands (D's), a settle after the typing has rested. Once a question.
  const landed = useRef("");
  useEffect(() => {
    if (!ask.answer || ask.title === landed.current) return;
    const timer = setTimeout(() => {
      landed.current = ask.title;
      if (!stillMotion()) flare(halo.current, 1.34);
    }, SETTLE_MS);
    return () => clearTimeout(timer);
  }, [ask.answer, ask.title]);

  function enterScope(next: GoToScope, typed: string) {
    setScope(next);
    setMadeFrom(sigilScope(typed) ? "" : typed);
    setQuery("");
    setMore(false);
    setActive(0);
    // One gesture, and it starts at the lamp. Never on leaving.
    const pane = root.current?.closest<HTMLElement>(".go-to-pane") ?? null;
    if (!stillMotion()) entered({ pane, scrim: pane?.parentElement ?? null, halo: halo.current, beam: beam.current, wave: wave.current });
  }
  function leaveScope(giveBack: boolean) {
    setScope(null);
    setQuery(giveBack ? madeFrom : "");
    setMadeFrom("");
    setActive(0);
    field.current?.focus();
  }
  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
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
      } else if (current.type === "answer" || (answered && (event.metaKey || event.ctrlKey))) {
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
      setMore(false);
      setActive(0);
    }
  }

  const placeholder = scope ? GO_TO_SCOPES[scope].hint : phone ? GO_TO_PHONE_PLACEHOLDER[audience] : GO_TO_PLACEHOLDER[audience];
  const answer = ask.answer?.panel;
  const typed = query.trim() !== "";

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
          aria-expanded={things.length > 0 || Boolean(answer)}
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
            // A scope's own key first becomes the scope, and is not kept as typing.
            if (sigil) return enterScope(sigil, GO_TO_SCOPES[sigil].sigil);
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

      <div
        ref={list}
        id={listId}
        role="listbox"
        aria-label="Where to go"
        className="go-to-list"
        hidden={!things.length && !asking && !(typed && !things.length)}
        onScroll={(event) => soften(event.currentTarget)}
      >
        {current && <span ref={point} aria-hidden className="go-to-point" />}
        {asking && answer && ask.href && (
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
            className="go-to-row hover-tint flex-col items-start! gap-1 py-2.5"
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
        {asking && !answer && (
          <div className="flex flex-col gap-0.5 py-2 pl-[calc(var(--go-to-ax)-var(--go-to-pad))] pr-3.5">
            <span className="font-display text-title text-ink">
              {ask.reading || ask.status?.lines[0]?.text === "Working it out."
                ? "Working it out."
                : ask.status
                  ? "Couldn't load this answer."
                  : "That isn't a question Syndra can answer."}
            </span>
            {!ask.reading && !ask.status && (
              <span className="text-small text-muted">She answers only what she can read exactly, and she never guesses. These she can:</span>
            )}
          </div>
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
              className="go-to-row hover-tint cursor-pointer"
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
              className="go-to-row hover-tint"
            >
              <span className="go-to-name">{thing.name}</span>
              <span className="min-w-0 flex-1 truncate text-small text-faint">{thing.descriptor}</span>
              <span className="flex-none text-small text-faint">{thing.tag}</span>
            </Link>
          ),
        )}
        {typed && !asking && !things.length && (
          <div className="flex flex-col gap-0.5 py-2 pl-[calc(var(--go-to-ax)-var(--go-to-pad))] pr-3.5">
            <span className="font-display text-title text-ink">
              Nothing by that name{scope ? ` among ${GO_TO_SCOPES[scope].word.toLowerCase()}` : ""}.
            </span>
            <span className="text-small text-muted">
              {scope ? "Backspace looks everywhere." : "Try fewer letters, or the start of any word in the name."}
            </span>
          </div>
        )}
      </div>
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

/** The lamp's halo flares once: D's, a flash long. */
function flare(halo: HTMLElement | null, to = 1.3) {
  halo?.animate?.([{ opacity: 0.72, scale: 1 }, { opacity: 1, scale: to, offset: 0.25 }, { opacity: 0.72, scale: 1 }], {
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

/**
 * The Input and its grouped list. Inside the dialog it is the whole surface;
 * inside the phone's Go to sheet (`embedded`) it sits above the page list,
 * which stays (as `idle`) until something is typed.
 */
export function GoToPanel({
  nav,
  onGo,
  embedded = false,
  idle,
}: {
  nav: NavEntry[];
  /** Called when a row is opened, so the host can close. */
  onGo: () => void;
  embedded?: boolean;
  /** Embedded only: what the sheet shows while the Input is empty. */
  idle?: React.ReactNode;
}) {
  const { audience } = useUiView();
  const { catalog, bundles } = useNameCatalog();
  const here = usePathname();
  // Read once, as it opens: the places opened before this one.
  const [lately] = useState(recallLately);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const base = useId();
  const listId = `${base}-list`;

  const groups = useMemo(
    () =>
      goTo(
        {
          nav,
          // A member's list is their own pages and nothing else; Basic has no Bundles page.
          names: audience !== "member",
          bundles: audience === "advanced",
          people: catalog?.users,
          projects: catalog?.projects,
          apps: catalog?.applications,
          bundleList: bundles,
          lately,
          here,
        },
        query,
      ),
    [nav, audience, catalog, bundles, query, lately, here],
  );
  const typed = query.trim() !== "";
  const showIdle = embedded && !typed;
  // The phone sheet's own list is its Pages: with nothing typed it keeps Lately above that list.
  const shown = showIdle ? groups.filter(({ group }) => group === "Lately") : groups;
  const items = flatItems(shown);
  const current = items.length ? items[Math.min(active, items.length - 1)] : undefined;
  const optionId = (item: GoToItem) => `${base}-${item.key.replace(/[^\w-]/g, "_")}`;

  const activeRef = useRef<HTMLAnchorElement | null>(null);
  useEffect(() => {
    activeRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [current?.key]);

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      if (!items.length) return;
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((index) => (Math.min(index, items.length - 1) + step + items.length) % items.length);
    } else if (event.key === "Enter" && current) {
      event.preventDefault();
      // Enter is a press on the selected link, so it navigates exactly as a click does.
      activeRef.current?.click();
    }
  }

  const field = (
    <Input
      type="search"
      bare={!embedded}
      // The whole row is the field: its inset is the input's own, so a press anywhere in the row lands in it.
      className={embedded ? "" : "px-6 py-3.5"}
      autoFocus={!embedded}
      role="combobox"
      aria-label={embedded ? "Go to by name" : "Go to"}
      aria-expanded={items.length > 0}
      aria-controls={listId}
      aria-autocomplete="list"
      aria-activedescendant={current ? optionId(current) : undefined}
      autoComplete="off"
      spellCheck={false}
      placeholder={GO_TO_PLACEHOLDER[audience]}
      value={query}
      onChange={(event) => {
        setQuery(event.target.value);
        setActive(0);
      }}
      onKeyDown={onKeyDown}
    />
  );

  return (
    // The phone's sheet keeps its boxed field inside its own padding; the dialog is the board's panel:
    // the field row, the list, the keys, edge to edge.
    <div className={`flex flex-col ${embedded ? "gap-3" : ""}`}>
      {embedded ? field : <div className="field-row sticky top-0 bg-surface-2">{field}</div>}

      {(!showIdle || items.length > 0) && (
        <div id={listId} role="listbox" aria-label="Where to go" className={`flex flex-col ${embedded ? "-mx-6" : "pb-1.5"}`}>
          {shown.map(({ group, items: rows }) => (
            <div key={group} role="group" aria-labelledby={`${base}-${group}`} className="flex flex-col">
              <div id={`${base}-${group}`} className="px-6 pb-1 pt-3">
                <span className="type-nav-group">{group}</span>
              </div>
              {rows.map((item) => {
                const selected = item.key === current?.key;
                return (
                  <Link
                    key={item.key}
                    id={optionId(item)}
                    ref={selected ? activeRef : undefined}
                    href={item.href}
                    replace={embedded}
                    role="option"
                    aria-selected={selected}
                    tabIndex={-1}
                    onClick={onGo}
                    className={`motion-tint flex min-h-[var(--tap)] items-center gap-3 px-6 py-1.5 ${selected ? "bg-tint-2" : ""}`}
                  >
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-body font-semibold text-ink">{item.name}</span>
                      {item.descriptor && <span className="truncate text-small text-muted">{item.descriptor}</span>}
                    </span>
                    <span className="flex-none text-fine text-faint">{item.kind}</span>
                  </Link>
                );
              })}
            </div>
          ))}
          {typed && groups.length === 0 && (
            <div className="px-6">
              <EmptyState title="Nothing by that name." />
            </div>
          )}
        </div>
      )}
      {showIdle && idle}

      {!embedded && (
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-divider px-6 py-2.5 text-fine text-faint">
          <span className="inline-flex items-center gap-1">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> move
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Enter</Kbd> open
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Esc</Kbd> {query !== "" ? GO_TO_ESC.typed : GO_TO_ESC.empty}
          </span>
        </p>
      )}
    </div>
  );
}

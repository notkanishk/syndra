"use client";

import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

/**
 * The arrive tokens, for the places a timer has to agree with CSS. Held to
 * globals.css by design-system.test.ts.
 */
export const ARRIVE_MS = 420; // --duration-arrive
export const ARRIVE_STEP_MS = 40; // --arrive-step
export const ARRIVE_CAP = 5; // --arrive-cap

/** Row five is the last to have a delay of its own; everything after arrives with it. */
export const ARRIVE_SETTLED_MS = ARRIVE_MS + (ARRIVE_CAP - 1) * ARRIVE_STEP_MS;

/** How long a Back, or a Show more, is remembered. A read can land seconds after the click. */
export const ARRIVE_MEMORY_MS = 5000;

// A screen arrives when you navigate to it (S11), and only then. Back and
// Forward, and a reload, return you to somewhere you have been: it is simply
// there. The browser says so with `popstate` (and the navigation entry's type
// for a reload); the page asked for is remembered by path, so a link followed
// straight after a Back still arrives.
// ponytail: a time window, not a router hook. A read slower than the window
// lands as a first paint; widen it if that is ever seen.
let returned: { path: string; at: number } | null = null;
const markReturned = () => {
  returned = { path: window.location.pathname, at: Date.now() };
};
if (typeof window !== "undefined") {
  window.addEventListener("popstate", markReturned);
  const entry = window.performance?.getEntriesByType?.("navigation")[0] as
    | PerformanceNavigationTiming
    | undefined;
  if (entry && entry.type !== "navigate") markReturned();
}

/** True when this mount is the reader coming back to a page, not going to it. */
export function returnedToThisPage(): boolean {
  return (
    returned !== null &&
    returned.path === window.location.pathname &&
    Date.now() - returned.at < ARRIVE_MEMORY_MS
  );
}

// Rows the reader asked for (Show 25 more, Show all) arrive; rows a refresh,
// a filter or a search brings simply appear. Only the reader's own click says
// which, so the click arms this and the next rows to land are the ones.
let armedAt = 0;
export const armArrive = () => {
  armedAt = Date.now();
};
/** True while rows the reader asked for are landing: they arrive, so nothing else announces them. */
export const armedRecently = () => Date.now() - armedAt <= ARRIVE_MEMORY_MS;

/**
 * A list's rows rise in on its first paint, and never again while it stays
 * mounted (A-05 §2.10).
 *
 * `arrive-list` is pure CSS on the children, so without this any child that
 * mounts later (a refresh bringing a row, a filter, a search) replayed the
 * rise, and the page moved in response to a background read. Once the stagger
 * has run the list is marked settled and the CSS stops animating children:
 * rows that stay hold still, new rows simply appear. A list that mounts anew
 * (navigating to the page, a fold opening) is a new first paint and arrives;
 * one that mounts because the reader went Back to its page does not.
 *
 * Rows the reader asked for (see `armArrive`) are the exception: they arrive,
 * staggered from the first new one, while the rows already there hold.
 *
 * Key the rows by id so rows that stay are not remounted.
 */
export function ArriveList({
  children,
  className = "contents",
  startSettled = false,
}: {
  children: ReactNode;
  /** `contents` by default: the wrapper must stay out of flex and grid layout. */
  className?: string;
  /** A list that has already arrived once in this place (its parent remounted it). */
  startSettled?: boolean;
}) {
  const [settled, setSettled] = useState(startSettled);
  const node = useRef<HTMLDivElement | null>(null);

  // After mount, not in the initial state: the server cannot know about a Back,
  // and a different first render is a hydration mismatch.
  useLayoutEffect(() => {
    if (returnedToThisPage()) setSettled(true);
  }, []);

  useEffect(() => {
    if (settled) return;
    const timer = setTimeout(() => setSettled(true), ARRIVE_SETTLED_MS);
    return () => clearTimeout(timer);
  }, [settled]);

  useEffect(() => {
    const list = node.current;
    if (!settled || !list) return;
    const watch = new MutationObserver((records) => {
      if (!armedRecently()) return;
      let index = 0;
      records.forEach((record) =>
        record.addedNodes.forEach((added) => {
          if (!(added instanceof HTMLElement)) return;
          added.style.setProperty("--i", String(index++));
          added.setAttribute("data-arriving", "");
          // A row's own children animate too, and their end bubbles up here.
          added.addEventListener("animationend", (event) => {
            if (event.target === added) added.removeAttribute("data-arriving");
          });
        }),
      );
    });
    watch.observe(list, { childList: true });
    return () => watch.disconnect();
  }, [settled]);

  return (
    <div
      ref={node}
      className={`${className} arrive-list`}
      data-settled={settled ? "" : undefined}
    >
      {children}
    </div>
  );
}

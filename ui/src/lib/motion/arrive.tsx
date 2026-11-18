"use client";

import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { forgetOpenedRow, handBackRow, moveBetweenRows, rowFocused, rowOpened, useRovingRows } from "@/lib/keys";
import { recallPaths, remember } from "@/lib/memory";

import { ARRIVE_MEMORY_MS, ARRIVE_SETTLED_MS } from "./still";

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

// A place arrives once a session, and never by key (S11, A-24 §2 and Q14).
// The first visit to a path in this tab's session arrives as built. A later
// visit, or one made by key (Enter on a link, a row opened from Go to), is
// quiet: the header tints in and the content is simply there, because an
// arrival that informed the first time is only latency the thirtieth. A Back,
// a Forward and a reload are quiet too, and stiller still (`returnedToThisPage`).
// The paths that have arrived live in sessionStorage; a new tab starts afresh.
// ponytail: every path is kept for the tab's life; a day of a hundred people
// pages is a few kilobytes. Trim it if a session ever grows past that.
const ARRIVED = "syndra:arrived";
let byKey: { path: string; at: number } | null = null;

/** The next visit to `href` was asked for by key, so it does not arrive (Go to, Enter on a link). */
export function visitByKey(href: string) {
  if (typeof window !== "undefined") byKey = { path: new URL(href, window.location.href).pathname, at: Date.now() };
}
if (typeof document !== "undefined") {
  // A link pressed with Enter is a click with no pointer: `detail` counts pointer clicks and is 0 here
  // (UI Events, `detail` for click). Captured, so it is noted before the router moves.
  document.addEventListener(
    "click",
    (event) => {
      const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
      if (link && event.detail === 0) visitByKey(link.getAttribute("href") ?? "");
    },
    true,
  );
}

// The page on screen, when it was shown, and whether this visit is quiet. The
// top bar notes it as it renders, which is before the page's own parts render
// in the same pass, so a part can ask whether it is part of the page's first
// paint or came after it.
let page: { path: string; at: number; quiet: boolean } | null = null;
export function notePage(path: string) {
  if (page?.path === path) return;
  const arrived = recallPaths("session", ARRIVED);
  const recent = (mark: { path: string; at: number } | null) =>
    mark !== null && mark.path === path && Date.now() - mark.at < ARRIVE_MEMORY_MS;
  page = { path, at: Date.now(), quiet: arrived.includes(path) || recent(byKey) || recent(returned) };
  if (!arrived.includes(path)) remember("session", ARRIVED, [...arrived, path]);
}
/** True when the visit on screen does not arrive: a place already seen this session, reached by key, or by Back. */
export const quietVisit = () =>
  page !== null && page.quiet && typeof window !== "undefined" && page.path === window.location.pathname;
/** True once this page's first paint is over (or no page was noted): what mounts now is late. */
export const pageSettled = () => page === null || Date.now() - page.at > ARRIVE_SETTLED_MS;

// Switching Basic and Advanced opens and closes parts of the page you are on
// by Unfold, and nothing arrives (A-08 §3): a list inside a part the switch
// opened, or one its first read fills a moment later, is simply there. Kept
// by path like `returned`, so a page reached after the switch still arrives.
let switched: { path: string; at: number } | null = null;
export const markViewSwitched = () => {
  switched = { path: window.location.pathname, at: Date.now() };
};
const switchedViewHere = () =>
  switched !== null &&
  switched.path === window.location.pathname &&
  Date.now() - switched.at < ARRIVE_MEMORY_MS;

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
 * one that mounts because the reader went Back to its page, or switched
 * Basic and Advanced on it, does not.
 *
 * Rows the reader asked for (see `armArrive`) are the exception: they arrive,
 * staggered from the first new one, while the rows already there hold.
 *
 * Key the rows by id so rows that stay are not remounted.
 *
 * It also carries the list's keys (A-19 P7): a row link marked `data-row` is a
 * stop for the arrow keys, with roving focus (`lib/keys.ts`). And it gives your
 * row back (A-24 §11.4): the row you opened is remembered, and when Back brings
 * you to this list again that row has the focus and keeps its tint for one flash.
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
  useRovingRows(node);

  // After mount, not in the initial state: the server cannot know about a Back,
  // and a different first render is a hydration mismatch.
  useLayoutEffect(() => {
    if (returnedToThisPage() || switchedViewHere() || quietVisit()) setSettled(true);
    // A visit that is not a Back has no row to give back, so an older visit's row never flashes.
    if (!returnedToThisPage()) forgetOpenedRow();
  }, []);

  // Every render until it lands: the rows come with a read, often after the mount.
  const handedBack = useRef(false);
  useEffect(() => {
    if (handedBack.current || !node.current || !returnedToThisPage()) return;
    handedBack.current = handBackRow(node.current);
  });

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
      onFocus={(event) => rowFocused(node.current, event.target)}
      onClick={(event) => rowOpened(event.target)}
      onKeyDown={(event) => moveBetweenRows(node.current, event)}
      className={`${className} arrive-list`}
      data-settled={settled ? "" : undefined}
    >
      {children}
    </div>
  );
}

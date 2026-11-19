"use client";

import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { ROW_STOP, moveBetweenRows, rowFocused, useRovingRows } from "@/lib/keys";
import { keepPlace, recallPlace, returnedToThisPage } from "@/lib/place";

import { flashOnce } from "./flash";
import { ARRIVE_MEMORY_MS, ARRIVE_SETTLED_MS } from "./still";

// A screen arrives every time you go to it (S11; the owner reversed A-24 Q14,
// 19 Nov 2026: "arrive on every visit"): a link, Go to and Enter alike, the
// header tinting in and what follows rising 40ms apart, capped at five. Back,
// Forward and a reload return you to somewhere you have been, and it is simply
// there: that is place's to know (`returnedToThisPage`, lib/place.ts), said
// here too. So are a filter, a sort and the Basic/Advanced switch.
export { returnedToThisPage };

// The page on screen and when it was shown. The top bar notes it as it renders,
// which is before the page's own parts render in the same pass, so a part can
// ask whether it is part of the page's first paint or came after it.
let page: { path: string; at: number } | null = null;
export function notePage(path: string) {
  if (page?.path === path) return;
  page = { path, at: Date.now() };
}
/** True once this page's first paint is over (or no page was noted): what mounts now is late. */
export const pageSettled = () => page === null || Date.now() - page.at > ARRIVE_SETTLED_MS;

// Switching Basic and Advanced opens and closes parts of the page you are on
// by Unfold, and nothing arrives (A-08 §3): a list inside a part the switch
// opened, or one its first read fills a moment later, is simply there. Kept
// by path like a Back, so a page reached after the switch still arrives.
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
    if (returnedToThisPage() || switchedViewHere()) setSettled(true);
    // A visit that is not a Back has no row to give back, so an older visit's row never flashes.
    if (!returnedToThisPage()) keepPlace("row", null);
  }, []);

  // Back gives you your row (A-24 §11.4): every render until it lands, as the rows come with a read.
  // It takes the focus without scrolling (the scroll is the place's own) and keeps its tint for one
  // flash, once: a reload after this is not another return to it.
  const handedBack = useRef(false);
  useEffect(() => {
    const list = node.current;
    if (handedBack.current || !list || !returnedToThisPage()) return;
    const href = recallPlace("row");
    const row = Array.from(list.querySelectorAll<HTMLElement>(ROW_STOP)).find((each) => each.getAttribute("href") === href);
    if (!row) return;
    handedBack.current = true;
    keepPlace("row", null);
    rowFocused(list, row);
    row.focus({ preventScroll: true });
    flashOnce(row);
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
      onClick={(event) => {
        // Only a plain primary click opens the row here: a modified or middle click opens it elsewhere.
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        const href = event.target instanceof Element ? event.target.closest(ROW_STOP)?.getAttribute("href") : null;
        if (href) keepPlace("row", href);
      }}
      onKeyDown={(event) => moveBetweenRows(node.current, event)}
      className={`${className} arrive-list`}
      data-settled={settled ? "" : undefined}
    >
      {children}
    </div>
  );
}

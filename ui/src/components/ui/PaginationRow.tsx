"use client";

import { useCallback, useState, type RefObject } from "react";

import { armArrive } from "@/components/ui/ArriveList";
import { Button } from "@/components/ui/Button";
import { stillMotion } from "@/lib/motion";

/** The first page of every long list, and what Show more adds (A-04 section 7.4). */
export const PAGE_FIRST = 10;
export const PAGE_MORE = 25;

/** The one limit every long list pages by. */
export function usePaging(initial = PAGE_FIRST, step = PAGE_MORE) {
  const [limit, setLimit] = useState(initial);
  return {
    limit,
    initial,
    step,
    more: useCallback(() => setLimit((n) => n + step), [step]),
    all: useCallback((total: number) => setLimit(total), []),
    back: useCallback(() => setLimit(initial), [initial]),
  };
}

/**
 * The one row under every long list: "Showing 10 of 61 · Show 25 more · Show
 * all 61 · Back to 10".
 *
 * With a `total` it is the whole sentence. Without one (a list the server hands
 * over a page at a time) it never promises a number it hasn't got: no "of N",
 * no Show all, and at the end it says the list has ended rather than letting a
 * button quietly stop appearing.
 */
export function PaginationRow({
  shown,
  total,
  hasMore = false,
  since,
  initial = PAGE_FIRST,
  step = PAGE_MORE,
  pending = false,
  onMore,
  onAll,
  onBack,
  anchor,
}: {
  shown: number;
  /** Absent for a server-fetched list that doesn't know how long it is. */
  total?: number;
  /** Without a total: whether the server has more to give. */
  hasMore?: boolean;
  /** Without a total, at the end: the short date the list reaches back to. */
  since?: string | null;
  initial?: number;
  step?: number;
  /** Show more is fetching: it carries its own busy state. */
  pending?: boolean;
  onMore: () => void;
  /** With a total: show everything. */
  onAll?: () => void;
  onBack: () => void;
  /** What Back to 10 returns to; the page heading when not given. */
  anchor?: RefObject<HTMLElement | null>;
}) {
  const known = total !== undefined;
  const remaining = known ? total - shown : 0;
  const ended = known ? remaining <= 0 : !hasMore;

  // A list that fits its first page has no paging to say.
  if (known && ended && total <= initial) return null;

  // The rows the reader asked for arrive; the ones already there hold (8c).
  const more = () => {
    armArrive();
    onMore();
  };
  const all = () => {
    armArrive();
    onAll?.();
  };

  // Glide to the heading first, then trim the list out of sight (8c): trimmed
  // first, the page would shrink under the reader's eyes and the scroll would
  // be a jump from wherever the browser clamped it.
  // ponytail: switch to glideTo once it lands.
  const back = () => {
    const target = anchor?.current ?? document.querySelector<HTMLElement>("h1");
    if (!target) return onBack();
    if (!target.hasAttribute("tabindex")) target.setAttribute("tabindex", "-1");
    target.focus({ preventScroll: true });
    if (stillMotion()) {
      target.scrollIntoView?.({ behavior: "auto", block: "start" });
      return onBack();
    }
    target.scrollIntoView?.({ behavior: "smooth", block: "start" });
    const glide = parseFloat(
      getComputedStyle(document.documentElement).getPropertyValue("--duration-glide"),
    );
    setTimeout(onBack, glide || 520);
  };

  const dot = (
    <span aria-hidden className="text-faint">
      ·
    </span>
  );

  return (
    <div className="row-divider flex flex-wrap items-center gap-x-3 gap-y-1 px-5 py-3 text-[13px] text-faint">
      {ended && !known && <span>{since ? `That's everything since ${since}` : "That's everything"}</span>}
      {ended && !known && dot}
      <span>
        {ended ? `Showing all ${shown}` : known ? `Showing ${shown} of ${total}` : `Showing ${shown}`}
      </span>
      {!ended && (
        <>
          {dot}
          <Button variant="ghost" size="sm" isPending={pending} onClick={more}>
            Show {known ? Math.min(step, remaining) : step} more
          </Button>
          {known && remaining > step && onAll && (
            <>
              {dot}
              <Button variant="ghost" size="sm" onClick={all}>
                Show all {total}
              </Button>
            </>
          )}
        </>
      )}
      {shown > initial && (
        <>
          {dot}
          <Button variant="ghost" size="sm" onClick={back}>
            Back to {initial}
          </Button>
        </>
      )}
    </div>
  );
}

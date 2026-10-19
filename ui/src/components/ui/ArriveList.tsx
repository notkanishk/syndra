"use client";

import { useEffect, useState, type ReactNode } from "react";

/** --duration-arrive (420ms) + the last staggered row's delay (--arrive-cap x --arrive-step). */
export const ARRIVE_SETTLED_MS = 420 + 5 * 40;

/**
 * A list's rows rise in on its first paint, and never again while it stays
 * mounted (A-05 §2.10).
 *
 * `arrive-list` is pure CSS on the children, so without this any child that
 * mounts later (a refresh bringing a row, a filter, a search) replayed the
 * rise, and the page moved in response to a background read. Once the stagger
 * has run the list is marked settled and the CSS stops animating children:
 * rows that stay hold still, new rows simply appear. A list that mounts anew
 * (navigating to the page, a fold opening) is a new first paint and arrives.
 *
 * Key the rows by id so rows that stay are not remounted.
 */
export function ArriveList({
  children,
  className = "contents",
}: {
  children: ReactNode;
  /** `contents` by default: the wrapper must stay out of flex and grid layout. */
  className?: string;
}) {
  const [settled, setSettled] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(true), ARRIVE_SETTLED_MS);
    return () => clearTimeout(timer);
  }, []);
  return (
    <div className={`${className} arrive-list`} data-settled={settled ? "" : undefined}>
      {children}
    </div>
  );
}

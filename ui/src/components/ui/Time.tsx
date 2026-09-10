"use client";

import { useEffect, useState } from "react";

import { formatClock, formatRelative, formatShortDate } from "@/lib/format";

/**
 * Anything whose value depends on "now" is rendered client-side only.
 *
 * The server and the browser do not share a clock or a locale, so formatting a
 * relative stamp in both places produces a hydration mismatch — React throws
 * the whole subtree away and re-renders it. The absolute date is stable, so it
 * is what the server sends; the relative reading appears on mount.
 */
export function Relative({ iso }: { iso: string | null | undefined }) {
  const [relative, setRelative] = useState<string | null>(null);

  useEffect(() => {
    setRelative(formatRelative(iso));
  }, [iso]);

  return <>{relative ?? formatShortDate(iso)}</>;
}

/**
 * The wall-clock time this page last looked. Client-only: on the server it is
 * a different second, in a different timezone, in a different locale.
 */
/**
 * "09:42:07" — the log ruler. Client-only for the same reason as the others:
 * the server renders in UTC and the browser in the operator's zone, and a
 * timestamp column that shifts by hours on hydration is worse than one that
 * appears a frame late.
 */
export function LogTime({ iso }: { iso: string | null | undefined }) {
  const [time, setTime] = useState<string | null>(null);

  useEffect(() => {
    if (!iso) return;
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return;
    setTime(date.toLocaleTimeString("en-GB", { hour12: false }));
  }, [iso]);

  return <>{time ?? "··:··:··"}</>;
}

export function ClockTime({ at }: { at?: number | null }) {
  const [time, setTime] = useState<string | null>(null);

  // Two things were wrong with the version this replaces, and the second is
  // the expensive one.
  //
  // It formatted with the BROWSER's locale while every other time in the
  // product uses `formatClock` — en-GB, 24-hour, fixed so a row never renders
  // "2:05" ambiguously between morning and afternoon. So one page said
  // "12:08 AM" where the rest of the product would say "00:08". Two clocks in
  // one product is the same defect as two counts.
  //
  // And it read `new Date()` on mount, under the words "last checked". That is
  // the moment the COMPONENT rendered, not the moment anything was read — a
  // freshness claim sourced from the render loop. It never ticked either, so
  // after an hour on the page it still named the minute you arrived. `at` is
  // the query's own `dataUpdatedAt`, which is when the answer actually came
  // back.
  useEffect(() => {
    setTime(formatClock(new Date(at ?? Date.now()).toISOString()));
  }, [at]);

  return <>{time ?? "just now"}</>;
}

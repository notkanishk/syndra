"use client";

import { useEffect, useState } from "react";

import { formatRelative, formatShortDate } from "@/lib/format";

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

export function ClockTime() {
  const [time, setTime] = useState<string | null>(null);

  useEffect(() => {
    setTime(
      new Date().toLocaleTimeString(undefined, {
        hour: "2-digit",
        minute: "2-digit",
      }),
    );
  }, []);

  return <>{time ?? "just now"}</>;
}

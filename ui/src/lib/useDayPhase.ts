"use client";

import { useLayoutEffect, useState } from "react";

import { dayPhase, type DayPhase } from "@/lib/greeting";

/** How often the part of the day is asked again; a state that did not change renders nothing. */
const ASK_EVERY_MS = 60_000; // not-motion: a clock's tick, once a minute

/**
 * The part of the day (`dayPhase`, the greeting's clock), on the client only:
 * null on the server and the first render, which has another clock
 * (hydration). It is asked again each minute, and re-renders only when it
 * changes: the greeting, the light and the quiet-hours sentence of Home or My access
 * turn together, even on a page left open across 22:00.
 */
export function useDayPhase(): DayPhase | null {
  const [phase, setPhase] = useState<DayPhase | null>(null);
  // Before paint: the light and the mark are drawn with the page, never a frame after it.
  useLayoutEffect(() => {
    setPhase(dayPhase(new Date()));
    const timer = setInterval(() => setPhase(dayPhase(new Date())), ASK_EVERY_MS);
    return () => clearInterval(timer);
  }, []);
  return phase;
}

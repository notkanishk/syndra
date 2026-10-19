"use client";

import { useEffect, useState } from "react";

/**
 * The motion tokens' durations, for the places a timer has to agree with CSS
 * (a closing thing stays mounted for exactly as long as its fade takes).
 * `design-system.test.ts` holds each to its token in globals.css.
 */
export const TINT_MS = 120;
export const SETTLE_MS = 260;
/** Closing's second beat: the scrim follows the sheet by this much (S1). */
export const LIGHT_LEAD_MS = 40;

/**
 * True when nothing should move: the reader asked for less motion, or this is a
 * test run that has not opted in to seeing motion (see test-utils/motion.ts).
 * Every exit phase and every drag asks this one question, so "both appear and
 * leave together, instantly" has one place to be true.
 */
export function stillMotion(): boolean {
  if (typeof window === "undefined") return true;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches) return true;
  return process.env.NODE_ENV === "test" && !(window as { __motionInTests?: boolean }).__motionInTests;
}

/**
 * Keeps something rendered for `ms` after `open` goes false, so it can fade
 * before it is gone (closing fades first, A-05). `leaving` is true during that
 * time; the caller draws the fade and ignores the pointer.
 */
export function useExitPhase(open: boolean, ms: number): { rendered: boolean; leaving: boolean } {
  const [present, setPresent] = useState(open);
  if (open && !present) setPresent(true);

  useEffect(() => {
    if (open || !present) return;
    if (stillMotion()) {
      setPresent(false);
      return;
    }
    const timer = setTimeout(() => setPresent(false), ms);
    return () => clearTimeout(timer);
  }, [open, present, ms]);

  const staying = present && !stillMotion();
  return { rendered: open || staying, leaving: !open && staying };
}

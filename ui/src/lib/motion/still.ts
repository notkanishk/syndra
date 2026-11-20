"use client";

import { useEffect, useState } from "react";

/**
 * The motion tokens' durations, for the places a timer has to agree with CSS
 * (a closing thing stays mounted for exactly as long as its fade takes).
 * `design-system.test.ts` holds each to its token in globals.css.
 */
export const TINT_MS = 120;
export const SETTLE_MS = 260;
export const PRESS_MS = 180;
export const BREATHE_MS = 2400;
/** Scrolling only: the one glide the page performs. */
export const GLIDE_MS = 520;
/** A closing region's gap folds this quickly: quicker than it opened. */
export const FOLD_MS = 200; // --duration-fold
/** Closing's second beat: the scrim follows the sheet by this much (S1). */
export const LIGHT_LEAD_MS = 40; // --light-lead
/** The door's light crossing (A-24 §3.2): a page lit later than this settles in rather than crossing. */
export const DOOR_OPEN_MS = 1800; // --door-open
/** A value that changed while you were looking elsewhere washes once for this long. */
export const FLASH_MS = 900; // --duration-flash
/** How long a press that landed keeps its bloom: flash's, and a beat. */
export const LANDED_HOLD_MS = FLASH_MS + 100;
/** How long a press is remembered once it settles, in case its read is still on its way. */
export const PRESS_FORGET_MS = 250;
/** A read that lands inside this never shows a skeleton at all (A-05 S7). */
export const SKELETON_DELAY_MS = 200;

/** The arrive tokens: a list's rise, its stagger and its cap. */
export const ARRIVE_MS = 420; // --duration-arrive
export const ARRIVE_STEP_MS = 40; // --arrive-step
export const ARRIVE_CAP = 5; // --arrive-cap
/** Row five is the last to have a delay of its own; everything after arrives with it. */
export const ARRIVE_SETTLED_MS = ARRIVE_MS + (ARRIVE_CAP - 1) * ARRIVE_STEP_MS;
/** How long a Back, or a Show more, is remembered. A read can land seconds after the click. */
export const ARRIVE_MEMORY_MS = 5000;
/** How long "Copied" stays before the control is Copy again (S18). */
export const COPIED_MS = 900;

/**
 * True when nothing should move: the reader asked for less motion, or this is a
 * test run that has not opted in to seeing motion (see test-utils/motion.ts).
 * Every exit phase and every drag asks this one question, so "both appear and
 * leave together, instantly" has one place to be true.
 */
export function stillMotion(): boolean {
  if (typeof window === "undefined") return true;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches) return true;
  return stillTint();
}

/**
 * True when even colour must not change over time: the server, or a test run
 * that has not opted in. Reduced motion is not a reason (A-24 Q15): it keeps
 * tint, so words changing in place still crossfade and nothing travels.
 */
export function stillTint(): boolean {
  if (typeof window === "undefined") return true;
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

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

/** A CSS `cubic-bezier(a, b, c, d)` as a function of progress, or null if it can't be read. */
function bezier(css: string): ((t: number) => number) | null {
  const m = css.match(/cubic-bezier\(\s*([-\d.]+)\s*,\s*([-\d.]+)\s*,\s*([-\d.]+)\s*,\s*([-\d.]+)\s*\)/);
  if (!m) return null;
  const [x1, y1, x2, y2] = m.slice(1).map(Number);
  const at = (a: number, b: number, s: number) => 3 * a * (1 - s) ** 2 * s + 3 * b * (1 - s) * s ** 2 + s ** 3;
  return (x) => {
    let lo = 0;
    let hi = 1;
    for (let i = 0; i < 24; i++) {
      const mid = (lo + hi) / 2;
      if (at(x1, x2, mid) < x) lo = mid;
      else hi = mid;
    }
    return at(y1, y2, (lo + hi) / 2);
  };
}

/** The landing heading's one wash: `.flash`, colour only, restarted if it is already on. */
function flashLanding(target: HTMLElement) {
  target.classList.remove("flash");
  void target.offsetWidth;
  target.classList.add("flash");
  target.addEventListener("animationend", () => target.classList.remove("flash"), { once: true });
}

let gliding = 0;

/**
 * The one glide the page performs (Motion 8d): scrolls to `target` over
 * `--duration-glide` on settle's curve, then the heading flashes once. Still,
 * it jumps and the flash stays. Every jump to a section, from a chip, the
 * pill, a count or a revealed panel, goes through here. The shell's own
 * scroller where there is one, so a narrow page never moves sideways.
 * `clear` is the space left above the heading. Resolves true once it has
 * landed (at once when still), false if a later jump took over.
 */
export function glideTo(target: HTMLElement, clear = 64): Promise<boolean> {
  const container = document.getElementById("app-scroll");
  const from = container ? container.scrollTop : window.scrollY;
  const base = container ? container.getBoundingClientRect().top : 0;
  const to = from + target.getBoundingClientRect().top - base - clear;
  const go = (top: number) => (container ? container.scrollTo?.({ top }) : window.scrollTo?.({ top }));

  const root = getComputedStyle(document.documentElement);
  const ms = parseFloat(root.getPropertyValue("--duration-glide"));
  const ease = bezier(root.getPropertyValue("--ease-settle"));
  const token = gliding + 1;
  gliding = token;

  if (stillMotion() || !(ms > 0) || !ease) {
    go(to);
    flashLanding(target);
    return Promise.resolve(true);
  }
  return new Promise((resolve) => {
    const start = performance.now();
    const step = (now: number) => {
      if (gliding !== token) return resolve(false); // a later jump has taken over
      const t = Math.min(1, (now - start) / ms);
      go(t < 1 ? from + (to - from) * ease(t) : to);
      if (t < 1) requestAnimationFrame(step);
      else {
        flashLanding(target);
        resolve(true);
      }
    };
    requestAnimationFrame(step);
  });
}

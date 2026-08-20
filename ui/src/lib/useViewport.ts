"use client";

import { useEffect, useState } from "react";

/**
 * The two breakpoints, as a question JavaScript can ask.
 *
 * Almost everything in the touch work is CSS, deliberately — one component
 * tree, one set of routes, reflow in the stylesheet. This exists for the small
 * set of decisions CSS genuinely cannot make: whether to open a keyboard on
 * mount, whether a control should be a sheet or a dialog in behaviour rather
 * than appearance.
 *
 * The queries mirror `--breakpoint-tablet` and `--breakpoint-desktop` in
 * globals.css. They are stated in `rem` for the same reason the tokens are:
 * a reader who has raised their platform's text size gets the earlier
 * breakpoint, which is what they were asking for.
 *
 * SSR-safe: the first render answers `false`, and the effect corrects it. The
 * callers are behavioural rather than structural, so a frame of the desktop
 * answer costs nothing — unlike a layout that would visibly reflow.
 */
function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(false);

  useEffect(() => {
    const list = window.matchMedia?.(query);
    if (!list) return;
    setMatches(list.matches);
    const onChange = (event: MediaQueryListEvent) => setMatches(event.matches);
    list.addEventListener("change", onChange);
    return () => list.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

/** Below the tablet breakpoint: one column, a tab bar, sheets. */
export function useIsPhone(): boolean {
  return useMediaQuery("(max-width: 44.99rem)");
}

/**
 * Below the desktop breakpoint — a phone or a tablet, both of which are a
 * thumb. This is the one to reach for when the question is "is this touch",
 * which is almost always the question.
 */
export function useIsTouch(): boolean {
  return useMediaQuery("(max-width: 67.49rem)");
}

/**
 * Whether this browser has asked for less movement.
 *
 * Read as a question rather than only obeyed in CSS, because two things in
 * this product are decided by it rather than styled by it: the drift chime is
 * silenced, and the control that offers the chime has to be able to say so.
 * A toggle reading "Sound on" while guaranteeing silence is exactly the kind
 * of claim this interface is not allowed to make.
 */
export function useReducedMotion(): boolean {
  return useMediaQuery("(prefers-reduced-motion: reduce)");
}

/**
 * Too little height to hold a consequence, a field and a keyboard at once.
 *
 * A phone in landscape is 390px tall, and a keyboard takes rather more than
 * half of it. Everything else about landscape needs no special case — 844px
 * of width is already past the tablet breakpoint, so the rail returns and
 * sheets become centred dialogs on their own — but the type-the-name gate
 * cannot fit, and the design's answer to not fitting is a rule rather than a
 * squeeze: the consequence sentence is the most protected element on the
 * screen and may not be the thing that scrolls away to make room.
 *
 * 26rem rather than a pixel value, so a reader who has raised their text size
 * crosses the threshold earlier, which is exactly when they should.
 */
export function useIsShort(): boolean {
  return useMediaQuery("(max-height: 26rem)");
}

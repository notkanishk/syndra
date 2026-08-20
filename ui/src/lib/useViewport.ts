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

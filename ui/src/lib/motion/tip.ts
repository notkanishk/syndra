"use client";

import { useEffect, useState } from "react";

/**
 * A second tooltip is instant (A-24 §11.11). The first one opens on its own
 * movement; once one is open, or has just shut, its neighbour opens with none,
 * because the reader is already reading definitions and the rise is only delay.
 * This is the "skip delay" every desktop and the WAI-ARIA tooltip pattern's
 * common implementations keep (Radix's `skipDelayDuration`, 300ms by default).
 */
export const TIP_WARM_MS = 300;

let openTips = 0;
let lastShut = 0;

/** Each test starts cold, as `resetDisplayLocaleForTests` does for the locale. */
export function resetTipsForTests() {
  openTips = 0;
  lastShut = 0;
}

/**
 * Whether this tip opened while another was open or had just shut: if so, it
 * appears at once, with no entrance. Decided as it opens and held until it shuts.
 */
export function useWarmTip(shown: boolean): boolean {
  const [warm, setWarm] = useState(false);
  const [was, setWas] = useState(shown);
  if (shown !== was) {
    setWas(shown);
    if (shown) setWarm(openTips > 0 || Date.now() - lastShut < TIP_WARM_MS);
  }
  useEffect(() => {
    if (!shown) return;
    openTips += 1;
    return () => {
      openTips -= 1;
      lastShut = Date.now();
    };
  }, [shown]);
  return warm;
}

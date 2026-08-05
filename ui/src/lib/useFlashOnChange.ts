"use client";

import { useEffect, useRef, useState } from "react";

/**
 * True for one `flash` while `value` differs from what it was last render.
 *
 * This exists because "changed while you were looking elsewhere" is the one
 * thing CSS cannot know. Everything else in the motion system is a class.
 *
 * Three properties, all deliberate:
 *
 *   - It never fires on arrival. A page that flashes every number as it
 *     lands has told the operator nothing about which one moved, and spends
 *     the signal at the exact moment it is worthless.
 *   - It watches the value, not the fetch. A poll that returns the same
 *     twelve unexplained grants is not news, and must not look like it.
 *   - Every change is marked, including one that lands while the previous
 *     mark is still playing.
 *
 * @param ready Whether `value` is real yet. Pass the inverse of a query's
 *   placeholder flag for anything backed by `placeholderData`: a fabricated
 *   zero is indistinguishable from a real one here, so the FIRST TRUE VALUE
 *   would otherwise read as a change from nothing to twelve and flash on
 *   every page load. Defaults to true for values that are always real.
 */
export function useFlashOnChange(value: unknown, ready = true): boolean {
  const previous = useRef(value);
  const wasReady = useRef(ready);
  const [flashing, setFlashing] = useState(false);

  useEffect(() => {
    // Only a change observed BETWEEN two real readings is news. This has to
    // consider the previous render as well as this one, because the commit
    // where placeholder data is replaced by the first real payload carries a
    // value change and a readiness change together — and that is an arrival.
    const betweenRealValues = wasReady.current && ready;
    wasReady.current = ready;

    if (Object.is(previous.current, value)) return;
    previous.current = value;
    if (!betweenRealValues) return;

    // The class has to LEAVE the element for CSS to replay the animation.
    // Setting a true state to true is a no-op in React, so a second change
    // inside the 900ms would restart the timer and nothing else — the row
    // would sit in an already-finished animation and the operator would never
    // see the second update. Dropping it for one frame is what makes each
    // change its own mark.
    let timer: ReturnType<typeof setTimeout>;
    setFlashing(false);
    const frame = requestAnimationFrame(() => {
      setFlashing(true);
      timer = setTimeout(() => setFlashing(false), FLASH_MS);
    });

    return () => {
      cancelAnimationFrame(frame);
      clearTimeout(timer);
    };
  }, [value, ready]);

  return flashing;
}

/**
 * How long the class stays on, in milliseconds.
 *
 * Must equal `--duration-flash` in globals.css: this only decides when the
 * class comes off, and the animation itself is authored there. If the two
 * drift, either the wash is cut short or the row sits in a finished animation
 * doing nothing. A design-system test holds them together.
 */
export const FLASH_MS = 900;

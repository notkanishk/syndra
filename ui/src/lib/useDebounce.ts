"use client";

import { useEffect, useState } from "react";

/**
 * Returns the latest value after `delayMs` of quiet. Used to throttle
 * search inputs and validate calls. Cleans up the timer on unmount.
 */
export function useDebounce<T>(value: T, delayMs = 250): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(t);
  }, [value, delayMs]);
  return debounced;
}

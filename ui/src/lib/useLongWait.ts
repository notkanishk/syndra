"use client";

import { useEffect, useState } from "react";

/**
 * Past ten seconds a person starts to doubt a wait (CRAFT §2.1, after Nielsen's
 * response-time limits), so a long wait says so, once (A-24 §6).
 */
export const LONG_WAIT_MS = 10_000;

/**
 * True once `pending` has gone `ms` without an answer; false again the moment
 * it settles. The caller changes one sentence, on tint: no bar, no counter,
 * because a check has no steps that could honestly be counted.
 */
export function useLongWait(pending: boolean, ms = LONG_WAIT_MS): boolean {
  const [long, setLong] = useState(false);
  useEffect(() => {
    if (!pending) {
      setLong(false);
      return;
    }
    const timer = setTimeout(() => setLong(true), ms);
    return () => clearTimeout(timer);
  }, [pending, ms]);
  // Settled is settled at once, not one render later when the effect has run.
  return pending && long;
}

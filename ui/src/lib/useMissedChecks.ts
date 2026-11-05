"use client";

import { useRef } from "react";

/**
 * The run of polls in a row that did not come back (A-20 P5). It is counted
 * from reads: a new answer from the server is one more poll, and only an
 * answer that says it is not reachable counts as a miss. A page that has not
 * read anything yet has seen no misses.
 *
 * `stamp` is anything that changes once per poll (React Query's
 * `dataUpdatedAt`); `reachable` is what that poll said, or undefined when it
 * has not answered.
 */
export function useMissedChecks(stamp: unknown, reachable: boolean | undefined): number {
  const seen = useRef<{ stamp: unknown; misses: number }>({ stamp: undefined, misses: 0 });
  if (reachable !== undefined && seen.current.stamp !== stamp) {
    seen.current = { stamp, misses: reachable ? 0 : seen.current.misses + 1 };
  }
  return seen.current.misses;
}

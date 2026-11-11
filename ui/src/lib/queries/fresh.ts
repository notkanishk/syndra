import { useContext, useEffect, useState } from "react";
import { QueryClientContext, type QueryKey } from "@tanstack/react-query";

import { ApiError, FRESH_READ_COOLDOWN, request } from "@/lib/api-client";

/**
 * Feature `freshness` (add-on contract §4), the UI half.
 *
 * A page load and every poll are plain GETs, answered from the add-on's copy.
 * "Read again" is the same GET with `?fresh=true`, and this is the only place
 * that adds it: the card's own `queryFn` never does, so a poll cannot send it.
 */

/** Why an answer is a copy, as the add-on said it. `retry_after` is whole seconds, at least 1. */
export interface Stale {
  reason: string;
  retry_after?: number;
  note?: string;
}

/**
 * The reasons this UI draws, as the add-on says them (the others, draining and
 * never read, draw as any copy does). Named here, never written as a
 * quoted value after `reason:`, which the repo guard reads as a fixed reason Syndra writes.
 */
export const STALE = { cached: "cached", cooldown: "cooldown", unreachable: "unreachable" } as const;

/** The same path, asking the target itself. Only the literal `true` is read by the backend. */
export function withFresh(path: string): string {
  return `${path}${path.includes("?") ? "&" : "?"}fresh=true`;
}

/** A read refused inside the cooldown, as the copy it leaves; undefined for any other error. */
export function cooldownOf(error: unknown): Stale | undefined {
  return error instanceof ApiError && error.code === FRESH_READ_COOLDOWN
    ? { reason: STALE.cooldown, retry_after: error.retryAfter }
    : undefined;
}

/**
 * One "Read again" for one card: the card's own GET, with `fresh=true`, whose
 * answer replaces what is cached under the card's key. It throws what the
 * backend refused (a 429 cooldown is `cooldownOf`), and keeps it in `error`
 * until the next press.
 */
export function useReadAgain<T>(queryKey: QueryKey, path: string) {
  // Not `useQueryClient`, which throws without a provider: a page that never presses needs none.
  const client = useContext(QueryClientContext);
  const [error, setError] = useState<unknown>(null);
  const [pending, setPending] = useState(false);
  async function press() {
    setError(null);
    setPending(true);
    try {
      client?.setQueryData(queryKey, await request<T>(withFresh(path)));
    } catch (e) {
      setError(e);
      throw e;
    } finally {
      setPending(false);
    }
  }
  return { press, pending, error };
}

/**
 * Whole seconds left of `seconds` from the moment it arrives, 0 once it is
 * over. A deadline, not a count of ticks, so a busy or backgrounded tab
 * catches up instead of running slow.
 */
export function useCountdown(seconds: number | undefined): number {
  const [left, setLeft] = useState(seconds ?? 0);
  useEffect(() => {
    const end = Date.now() + (seconds ?? 0) * 1000;
    const sync = () => setLeft(Math.max(0, Math.ceil((end - Date.now()) / 1000)));
    sync();
    if (!seconds) return;
    const id = setInterval(() => {
      sync();
      if (Date.now() >= end) clearInterval(id);
    }, 1000);
    return () => clearInterval(id);
  }, [seconds]);
  return left;
}

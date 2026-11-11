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
  /** When this answer arrived (ms since the epoch): a cooldown counts from here, not from when it was drawn. */
  since?: number;
  /** The reader asked for the read that was refused (a press), so the wait is theirs: only then is it drawn amber. */
  pressed?: boolean;
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
export function cooldownOf(error: unknown, since?: number): Stale | undefined {
  return error instanceof ApiError && error.code === FRESH_READ_COOLDOWN
    ? { reason: STALE.cooldown, retry_after: error.retryAfter, since }
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
  const [errorAt, setErrorAt] = useState<number>();
  const [pending, setPending] = useState(false);
  async function press() {
    setError(null);
    setPending(true);
    try {
      // A poll still in flight would land after this answer and put the copy back. Only a copy is
      // worth protecting: cancelling a first load with nothing cached reverts it to idle, with no data
      // and no error, which a list draws as empty.
      if (client?.getQueryData(queryKey) !== undefined) await client.cancelQueries({ queryKey });
      client?.setQueryData(queryKey, await request<T>(withFresh(path)));
    } catch (e) {
      setError(e);
      setErrorAt(Date.now());
      throw e;
    } finally {
      setPending(false);
    }
  }
  // A press the cooldown refused: the wait is the pressing reader's own.
  const refused = cooldownOf(error, errorAt);
  return { press, pending, error, errorAt, cooldown: refused && { ...refused, pressed: true } };
}

/**
 * Whole seconds left of `seconds` counted from `since` (when the answer
 * arrived; the moment of drawing if not given), 0 once it is over. A deadline,
 * not a count of ticks, so a busy or backgrounded tab catches up instead of
 * running slow, and a page that is left and come back to does not start over.
 */
export function useCountdown(seconds: number | undefined, since?: number): number {
  const secondsLeft = (end: number) => Math.max(0, Math.ceil((end - Date.now()) / 1000));
  const [left, setLeft] = useState(() => secondsLeft((since ?? Date.now()) + (seconds ?? 0) * 1000));
  useEffect(() => {
    const end = (since ?? Date.now()) + (seconds ?? 0) * 1000;
    const sync = () => setLeft(secondsLeft(end));
    sync();
    if (!seconds) return;
    const id = setInterval(() => {
      sync();
      if (Date.now() >= end) clearInterval(id);
    }, 1000);
    return () => clearInterval(id);
  }, [seconds, since]);
  return left;
}

import { useContext, useEffect, useRef, useState } from "react";
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
  /**
   * `false` where a page load ran into the wait, not a press: the reader did not cause it, so it is not
   * drawn amber. Absent means the reader did: a cooldown only reaches the data through a press, because
   * a load and a poll never send `fresh`. Marked at the load sites only.
   */
  pressed?: false;
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

/** A copy as of `since` (when its answer arrived), so a cooldown it carries counts from there and a remount does not restart it. */
export function stamped(stale: Stale | undefined, since?: number): Stale | undefined {
  return stale && since ? { ...stale, since } : stale;
}

/** The one marker: a cooldown a page load ran into, which the reader did not cause. */
export const pageLoad = (stale: Stale | undefined): Stale | undefined => stale && { ...stale, pressed: false };

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
  return { press, pending, error, errorAt, cooldown: cooldownOf(error, errorAt) };
}

/**
 * Whole seconds left of `seconds` counted from `since` (when the answer
 * arrived; the moment of drawing if not given), 0 once it is over. A deadline,
 * not a count of ticks, so a busy or backgrounded tab catches up instead of
 * running slow, and a page that is left and come back to does not start over.
 */
export function useCountdown(seconds: number | undefined, since?: number): number {
  const secondsLeft = (end: number) => Math.max(0, Math.ceil((end - Date.now()) / 1000));
  // A `since` of 0 is a query that has not answered, never the epoch.
  const from = since || undefined;
  const [left, setLeft] = useState(() => secondsLeft((from ?? Date.now()) + (seconds ?? 0) * 1000));
  useEffect(() => {
    const end = (from ?? Date.now()) + (seconds ?? 0) * 1000;
    const sync = () => setLeft(secondsLeft(end));
    sync();
    if (!seconds) return;
    const id = setInterval(() => {
      sync();
      if (Date.now() >= end) clearInterval(id);
    }, 1000);
    return () => clearInterval(id);
  }, [seconds, from]);
  return left;
}

/** The longest a press waits out a cooldown: the drift sweep's own ceiling (sweepWaitCeiling, 2 × constants.cooldown_default). */
export const RETRY_CEILING_S = 60;

/**
 * A person's press the add-on's cooldown refused, waited out and sent once more by itself (RFC 9110
 * §10.2.3, Retry-After): "Check now" and the dormant removal, so one press completes on a live read
 * (owner Q1). `press` is the person's own press; `refused` schedules the one retry after `retry_after`
 * (at most RETRY_CEILING_S) and returns false when this press already had it, so the second refusal
 * is shown as it is. `cancel` stops the wait; leaving the page does too. `left` counts it down.
 */
export function useRetryOnce() {
  const [wait, setWait] = useState<{ seconds: number; since: number } | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const retried = useRef(false);
  const left = useCountdown(wait?.seconds, wait?.since);
  useEffect(() => () => clearTimeout(timer.current), []);
  function cancel() {
    clearTimeout(timer.current);
    timer.current = undefined;
    setWait(null);
  }
  return {
    waiting: wait !== null,
    left,
    cancel,
    press(send: () => void) {
      cancel();
      retried.current = false;
      send();
    },
    refused(retryAfter: number | undefined, send: () => void): boolean {
      if (retried.current) return false;
      retried.current = true;
      const seconds = Math.min(Math.max(1, retryAfter ?? 1), RETRY_CEILING_S);
      setWait({ seconds, since: Date.now() });
      timer.current = setTimeout(() => {
        timer.current = undefined;
        setWait(null);
        send();
      }, seconds * 1000);
      return true;
    },
  };
}

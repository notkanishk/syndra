"use client";

import { QueryClient, QueryClientContext, queryOptions, useQuery } from "@tanstack/react-query";
import { useContext, useEffect, useRef } from "react";

import { request } from "@/lib/api-client";
import { playDriftChime } from "@/lib/driftChime";

/**
 * The four sidebar badge scalars.
 *
 * The rail must NOT consume /governance/summary: that payload carries every
 * pending request and expiring grant object, and the rail needs four integers.
 * This endpoint exists so the badges can be refreshed often and cheaply.
 */
export interface Indicators {
  pending_requests: number;
  expiring_grants: number;
  pending_propagation: number;
  drift: number;
  /**
   * Access somebody decided to withdraw that has not been withdrawn. A subset
   * of pending_propagation, counted apart from it because the two mean opposite
   * things about urgency: a queued grant is somebody waiting, and a queued
   * revocation is somebody still holding what was taken away.
   */
  unconfirmed_revocations: number;
  /**
   * True when at least one of them is a finding rather than a queue depth —
   * spent, or old enough that it is stuck rather than draining. The badge
   * changes on this and not on the count, because a count cannot carry the
   * difference and an operator reading "3" cannot tell.
   */
  revocations_escalated: boolean;
  /**
   * Holds past their review date and still in force. Counted apart from
   * expiring_grants, never summed with it: an expiring grant lapses if nobody
   * acts and a hold stays, so one number covering both would be counting
   * "access about to end" together with "access still being withheld".
   */
  holds_due: number;
  /**
   * Per connected system, the changes it never answered that wait for a person
   * to say what happened (A-09 §3). Never passwords (A-13 §5). A system with
   * none is absent.
   */
  unconfirmed_operations?: Record<string, number>;
  /**
   * Of pending_propagation, the changes Zitadel didn't take that Syndra sends
   * again on its own (the one retry model), and the ones it gave up on this
   * week. Said on Home and the rail, so neither a retry nor its end is silent.
   */
  retrying?: number;
  gave_up?: number;
  /** The same two, per system the change was for ("zitadel", "truenas"): a TrueNAS change is never called Zitadel's. */
  retrying_by?: Record<string, number>;
  gave_up_by?: Record<string, number>;
  /**
   * Whether this server sends changes again on its own (the background drain,
   * REVOCATION_DRAIN_ENABLED). Off, a change Zitadel didn't take waits for Send.
   */
  auto_retry?: boolean;
  zitadel_reachable: boolean;
}

const EMPTY: Indicators = {
  pending_requests: 0,
  expiring_grants: 0,
  pending_propagation: 0,
  drift: 0,
  unconfirmed_revocations: 0,
  holds_due: 0,
  revocations_escalated: false,
  zitadel_reachable: true,
};

/**
 * The rail's count, set from the answer (A-20 P8). Called in a mutation's
 * `onSuccess` and nowhere else, so the number changes when the server has
 * answered and never ahead of it; the refetch the same handler asks for then
 * confirms it. `change` returns the deltas to apply to the real reading, and
 * the result never goes below zero. With no reading in the cache yet there is
 * nothing to correct, so nothing is written (the fabricated zeros are not one).
 */
export function settleIndicators(
  client: QueryClient,
  change: Partial<Record<"pending_requests" | "expiring_grants" | "pending_propagation" | "drift" | "holds_due", number>>,
) {
  client.setQueryData<Indicators>(["governance", "indicators"], (current) => {
    if (!current) return current;
    const next = { ...current };
    for (const [key, delta] of Object.entries(change) as Array<[keyof typeof change, number]>) {
      next[key] = Math.max(0, current[key] + delta);
    }
    return next;
  });
}

function indicatorsQuery(enabled: boolean) {
  return queryOptions({
    queryKey: ["governance", "indicators"],
    queryFn: () => request<Indicators>("/governance/indicators"),
    enabled,
    // Frequent enough that a badge is never stale for long, cheap enough that
    // it costs four integers to be right.
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
    // A failed poll must never blank the rail: a badge that flickers to zero
    // reads as "the queue cleared", which is a lie with consequences.
    //
    // The cost of that safety is that the rail holds four fabricated zeros
    // before the first payload lands, and nothing downstream can tell them
    // from real ones. Anything that reacts to a CHANGE in these numbers has
    // to know which readings are real — see `ready` below and in
    // `useFlashOnChange`.
    placeholderData: (previous) => previous ?? EMPTY,
  });
}

/**
 * The same read, for a screen that reports a count and must not chime: the rail
 * owns the drift chime, and a second observer with its own memory would sound
 * it twice. The phone's tab bar is one such screen: it is mounted beside the
 * rail, not instead of it. `isPlaceholderData` tells a real zero from the fabricated one.
 */
export function useIndicatorsRead(enabled = true) {
  return useQuery(indicatorsQuery(enabled));
}

/**
 * Whether this server sends changes again on its own (the background drain).
 * True until read, the shipped default; false only when the server says so.
 */
export function useAutoRetry(): boolean {
  // Copy deep in a dialog reads this, and a dialog may render with no provider
  // above it (a unit render): that is the shipped default, never a throw.
  const client = useContext(QueryClientContext);
  return useQuery(indicatorsQuery(client !== undefined), client ?? NO_CLIENT).data?.auto_retry !== false;
}

/** Stands in for a missing provider; its query is disabled, so it never fetches. */
const NO_CLIENT = new QueryClient();

/**
 * Whether the rail's own poll could reach Zitadel; undefined until a real
 * reading lands (the placeholder's `true` is not one). The same read the rail
 * already makes, so asking costs nothing, and it is the answer Pending changes
 * and Home's pending card were already acting on while the shell said nothing.
 */
export function useZitadelReachable(enabled: boolean): boolean | undefined {
  const query = useQuery(indicatorsQuery(enabled));
  return query.isPlaceholderData ? undefined : query.data?.zitadel_reachable;
}

export function useIndicators(enabled: boolean) {
  const query = useQuery(indicatorsQuery(enabled));

  // The chime belongs to whoever learns the drift count first, which is this
  // poll. It fires only on a RISE — a count that stays at 12 across polls is
  // not news, and a chime on every poll would be trained out within an hour.
  const previousDrift = useRef<number | null>(null);
  const drift = query.data?.drift;
  // The placeholder's zero is not a reading, and skipping it here is what
  // keeps `previousDrift` null until a real payload lands. Without that the
  // first real count is a rise from a number nobody ever saw, and the chime
  // sounds on every page load — precisely the training-out it exists to avoid.
  const real = drift !== undefined && !query.isPlaceholderData;
  useEffect(() => {
    if (!real) return;
    const before = previousDrift.current;
    previousDrift.current = drift!;
    if (before !== null && drift! > before) playDriftChime();
  }, [drift, real]);

  return query;
}

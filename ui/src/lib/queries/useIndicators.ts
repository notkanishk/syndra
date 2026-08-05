"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

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
  zitadel_reachable: boolean;
}

const EMPTY: Indicators = {
  pending_requests: 0,
  expiring_grants: 0,
  pending_propagation: 0,
  drift: 0,
  zitadel_reachable: true,
};

export function useIndicators(enabled: boolean) {
  const query = useQuery({
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

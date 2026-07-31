"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * One direct grant approaching its expiry. `granted_by` and `created_at` are
 * what turn a row from "something expires" into "P. Raghunathan gave this on
 * 4 Jul" — which is the difference between extending confidently and going to
 * ask someone first.
 */
export interface ExpiringGrantRow {
  id: string;
  user_id: string;
  project_id: string;
  role_key: string;
  granted_by: string;
  reason: string;
  expires_at?: string | null;
  created_at: string;
}

const KEYS = {
  expiring: (days: number) => ["review", "expiring-grants", days] as const,
};

/**
 * Review › Expiring access reads its own window rather than a slice of the
 * governance summary. Today's queue looks 14 days ahead because its job is to
 * be finishable; this screen looks 30, because its job is a review.
 */
export function useExpiringGrants(withinDays = 30) {
  return useQuery({
    queryKey: KEYS.expiring(withinDays),
    queryFn: async () =>
      (
        await request<{ within_days: number; grants: ExpiringGrantRow[] }>(
          `/review/expiring-grants?within_days=${withinDays}`,
        )
      ).grants ?? [],
  });
}

export const expiringAccessQueryKeys = KEYS;

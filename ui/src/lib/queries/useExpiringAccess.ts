"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * An operator's recorded "I have seen this and I am letting it lapse".
 *
 * Present on a row only while it still applies. The backend stores the expiry date the decision
 * was made against and compares on read, so a grant somebody has since extended arrives here with
 * no acknowledgement at all — the row is undecided again, because the thing that was decided no
 * longer exists.
 */
export interface ExpiryAcknowledgement {
  by: string;
  at: string;
  note?: string;
}

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
  acknowledged?: ExpiryAcknowledgement | null;
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

export interface AcknowledgeExpiryInput {
  grantId: string;
  /**
   * The expiry the operator was looking at. Sent, not implied: the backend refuses an
   * acknowledgement of a date the grant no longer carries, so a page loaded before somebody
   * extended the grant gets a 409 telling it to reload rather than a write that silently never
   * applies.
   */
  expiresAt: string;
  note?: string;
}

/**
 * Records "seen, and letting it lapse". Changes no access — the expiry sweep still removes the
 * grant on its date — so nothing outside this queue needs invalidating.
 */
export function useAcknowledgeExpiry() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ grantId, expiresAt, note }: AcknowledgeExpiryInput) =>
      request<{ message: string }>(`/review/expiring-grants/${grantId}/acknowledge`, {
        method: "POST",
        body: { expires_at: expiresAt, note: note ?? "" },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["review", "expiring-grants"] });
    },
  });
}

/** Takes an acknowledgement back, returning the row to the undecided part of the queue. */
export function useClearExpiryAcknowledgement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (grantId: string) =>
      request<{ message: string }>(`/review/expiring-grants/${grantId}/acknowledge`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["review", "expiring-grants"] });
    },
  });
}

export const expiringAccessQueryKeys = KEYS;

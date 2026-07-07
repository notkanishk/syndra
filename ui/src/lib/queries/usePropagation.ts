"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

import { governanceQueryKeys } from "./useGovernance";

export interface PendingRow {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  source: string;
  source_ref?: string;
  status: string;
  attempts: number;
  created_at: string;
  last_error?: string;
}

export interface DrainResult {
  applied: number;
  failed: number;
  requeued: number;
  /**
   * Rows whose Zitadel outcome was decided but whose state could NOT be
   * persisted (mark/requeue/ledger-reconcile failed). Such a row stays in_flight
   * and is neither applied nor failed — the next drain reclaims it. A non-zero
   * value means "state write failed; resume again to retry", even on HTTP 200.
   */
  errored: number;
  halted: boolean;
  reason?: string;
}

const KEYS = {
  pending: ["propagations", "pending"] as const,
};

/** The operator's "awaiting Zitadel" worklist (pending + in_flight rows). */
export function usePendingPropagations() {
  return useQuery({
    queryKey: KEYS.pending,
    queryFn: async () =>
      (await request<{ pending: PendingRow[] }>("/propagations")).pending ?? [],
  });
}

/**
 * The operator's "Resume now" action: drains the outbox to Zitadel. On success
 * it invalidates both the pending list and the governance summary so the
 * sidebar badge and dashboard callout reflect the new depth immediately.
 */
export function useDrainPropagations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      request<DrainResult>("/propagations/drain", { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.pending });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

export const propagationQueryKeys = KEYS;

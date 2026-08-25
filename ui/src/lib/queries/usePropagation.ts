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
  /** Shared by every write one triggering event produced. Empty on older rows. */
  cascade_id?: string;
  status: string;
  attempts: number;
  created_at: string;
  last_error?: string;
}

/** Named for the screen that reads it, where "propagation" is never spoken. */
export type PendingPropagationRow = PendingRow;

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
  /**
   * Rows this pass made terminal because their retry budget ran out. A subset
   * of `failed`, reported apart from it because what an operator does next
   * differs: an ordinary failure names what the target said, and this one says
   * nobody will try again.
   */
  exhausted?: number;
  halted: boolean;
  reason?: string;
  /**
   * Whose pass produced `reason`. Its own field rather than a prefix on the
   * string, because the reasons are matched by callers as well as read.
   */
  halted_target?: string;
  /**
   * One entry per target the drain ran a pass for.
   *
   * A drain dispatches every registered target through its own dispatcher, and
   * the passes fail independently: a Zitadel outage halts Zitadel's pass and
   * leaves a reachable NAS's queue moving. So "halted" beside "applied: 9" is an
   * ordinary result rather than a contradiction, and the only way to read it is
   * per target.
   */
  passes?: DrainPass[];
}

export interface DrainPass {
  target: string;
  applied: number;
  failed: number;
  requeued: number;
  abandoned: number;
  errored: number;
  exhausted?: number;
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

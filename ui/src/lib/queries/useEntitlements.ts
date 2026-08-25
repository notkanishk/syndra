"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Converging a cohort's entitlements on a target: rehearse, then apply the id
 * the rehearsal issued.
 *
 * The apply carries the PLAN ID and not the original submission. That is the
 * whole point of the mechanism — an apply that resubmitted the request would
 * recompute a diff against a world that moved between the two requests, which
 * is the gap plan-then-apply closes.
 */

export interface EntitlementOutcome {
  user_id: string;
  name?: string;
  email?: string;
  effect: "apply" | "no_change" | "blocked";
  detail: string;
  consequence?: string;
}

export interface EntitlementPlan {
  op: string;
  plan_id?: string;
  applied: boolean;
  outcomes: EntitlementOutcome[];
  summary: {
    total: number;
    apply: number;
    no_change: number;
    blocked: number;
    failed: number;
    succeeded: number;
    queued: number;
  };
  /** Computed against the add-on's last-known state because the target was down. */
  provisional: boolean;
  /** When that state was read. The age a provisional plan must be labelled with. */
  state_read_at: string;
  truncated?: boolean;
}

export function useRehearseEntitlements(target: string) {
  return useMutation({
    mutationFn: (input: { subjectIds: string[]; acknowledgeScope?: boolean }) =>
      request<EntitlementPlan>(`/targets/${target}/entitlements/rehearse`, {
        method: "POST",
        body: { subject_ids: input.subjectIds, acknowledge_scope: input.acknowledgeScope ?? false },
      }),
  });
}

export interface EntitlementApplyResult {
  plan_id: string;
  target: string;
  provisional: boolean;
  queued: Array<{ subject_id: string; outbox_id: string }>;
  summary: {
    total: number;
    queued: number;
    no_change: number;
    blocked: number;
    /** Always zero from this endpoint. Nothing here has reached the target. */
    succeeded: number;
  };
}

export function useApplyEntitlements(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { planId: string; subjectIds: string[] }) =>
      request<EntitlementApplyResult>(`/targets/${target}/entitlements/apply`, {
        method: "POST",
        body: { plan_id: input.planId, subject_ids: input.subjectIds },
      }),
    onSuccess: () => {
      // The rows are queued, so what changed is the pending count — never the
      // target's own state, which the drain has not touched yet.
      client.invalidateQueries({ queryKey: ["governance", "indicators"] });
      client.invalidateQueries({ queryKey: ["propagation"] });
    },
  });
}

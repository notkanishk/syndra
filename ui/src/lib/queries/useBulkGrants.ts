"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Bulk access changes, rehearsed before they land.
 *
 * The same request body serves both passes — rehearsal is the absence of
 * `?apply=true`, not a different payload — so the plan an operator approves and
 * the write it authorises cannot describe different operations. The backend
 * re-rehearses on apply regardless; this is a UX guarantee, not the security
 * boundary.
 */

export type BulkOp =
  | "assign_role"
  | "remove_role"
  | "assign_bundle"
  | "remove_bundle"
  | "extend";

export interface BulkGrantInput {
  op: BulkOp;
  user_ids: string[];
  project_id?: string;
  role_key?: string;
  bundle_id?: string;
  reason: string;
  duration_days?: number;
}

/**
 * One person's row in the plan.
 *  - `apply`     — will change (rehearsal)
 *  - `applied`   — did change (result)
 *  - `no_change` — already in the target state
 *  - `blocked`   — refused, with a reason. Never silently dropped.
 *  - `failed`    — attempted and errored
 */
export type BulkEffect = "apply" | "applied" | "no_change" | "blocked" | "failed";

export interface BulkOutcome {
  user_id: string;
  name: string;
  email: string;
  effect: BulkEffect;
  detail: string;
  /** What this person is left holding — the part a count never tells you. */
  consequence?: string;
  grant_ids?: string[];
}

export interface BulkSummary {
  total: number;
  apply: number;
  no_change: number;
  blocked: number;
  failed: number;
  succeeded: number;
}

export interface BulkPlan {
  op: BulkOp;
  applied: boolean;
  outcomes: BulkOutcome[];
  summary: BulkSummary;
}

/** Ceiling mirrors the backend's `BulkMaxUsers`. */
export const BULK_MAX_USERS = 500;

function bulkPath(apply: boolean): string {
  return apply ? "/grants/bulk?apply=true" : "/grants/bulk";
}

/** Rehearse: computes the plan, writes nothing. */
export function useRehearseBulk() {
  return useMutation({
    mutationFn: (input: BulkGrantInput) =>
      request<BulkPlan>(bulkPath(false), { method: "POST", body: input }),
  });
}

/**
 * Apply. Invalidates everything a bulk write can move — a stale People row
 * showing pre-bulk access is how an operator ends up running the same batch
 * twice.
 */
export function useApplyBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BulkGrantInput) =>
      request<BulkPlan>(bulkPath(true), { method: "POST", body: input }),
    onSuccess: () => {
      for (const key of ["users", "roles", "bundles", "governance", "propagations", "audit"]) {
        qc.invalidateQueries({ queryKey: [key] });
      }
    },
  });
}

/** Verb for a headline: "Grant Laser Lab / trained to 47 people". */
export function describeBulkOp(op: BulkOp): string {
  switch (op) {
    case "assign_role":
      return "Grant a role";
    case "remove_role":
      return "Remove a role";
    case "assign_bundle":
      return "Add to a bundle";
    case "remove_bundle":
      return "Remove from a bundle";
    case "extend":
      return "Extend expiring access";
  }
}

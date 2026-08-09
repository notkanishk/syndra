"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Bulk access changes, rehearsed before they land.
 *
 * The rehearsal returns a `plan_id`: the backend persisted what it showed, and
 * applying cites that id. It is no longer a UX guarantee resting on the client
 * re-sending the same thing — the backend executes the rows it recorded, and a
 * body that does not match the one the plan was computed for is refused.
 *
 * The body is still sent on apply, because it carries what the write needs (the
 * project, the role, the duration). It is bound, not trusted.
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
  /** Cites the rehearsal being applied. Set by the apply pass, never composed by hand. */
  /**
   * The operator confirming an affected-subject count above the configured
   * limit. Sent on the rehearsal, never on the apply: it unlocks issuing the
   * approval rather than changing what the approval does.
   */
  acknowledge_scope?: boolean;
  plan_id?: string;
  /**
   * Narrows `extend` to specific grants. Omit it to extend every expiring direct grant the named
   * people hold — which is what a screen that selects PEOPLE means.
   *
   * Review › Expiring access selects grant ROWS, so it must pass them: reducing those rows to
   * user ids renews grants the operator never saw, including ones outside that screen's window.
   */
  grant_ids?: string[];
}

/**
 * One person's row in the plan.
 *  - `apply`     — will change (rehearsal)
 *  - `applied`   — did change (result)
 *  - `no_change` — already in the target state
 *  - `blocked`   — refused, with a reason. Never silently dropped.
 *  - `failed`    — attempted and errored
 *  - `queued`    — Syndra recorded it; Zitadel has not confirmed it yet
 *
 * `applied` and `queued` are the distinction that matters after the fact.
 * "Applied" means Zitadel confirmed the change; "queued" means the records here
 * are updated and the change has not reached Zitadel — the drain was refused,
 * halted, or the bundle applies on confirmation. Collapsing the two tells an
 * operator a door is locked while it is open.
 */
export type BulkEffect = "apply" | "applied" | "no_change" | "blocked" | "failed" | "queued";

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
  /** Recorded, not yet confirmed upstream. Never folded into `succeeded`. */
  queued: number;
}

export interface BulkPlan {
  op: BulkOp;
  /**
   * The approval this rehearsal became. Present on a rehearsal, and what an
   * apply must cite; absent on a result, which is a report rather than an
   * approval.
   */
  plan_id?: string;
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
      // `review` is here because `extend` rewrites expiry dates, and Review › Expiring access is
      // built entirely from those — a bulk extend launched from that screen would otherwise leave
      // every row it just renewed on screen with its old date.
      for (const key of [
        "users",
        "roles",
        "bundles",
        "governance",
        "propagations",
        "audit",
        "review",
      ]) {
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

"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface ExpiringGrant {
  id: string;
  user_id: string;
  project_id: string;
  role_key: string;
  expires_at?: string | null;
}

export interface PendingPropagationSummary {
  count: number;
  zitadel_reachable: boolean;
  last_queued_at?: string | null;
}

export interface DriftItem {
  id: string;
  /** Which system the access was found on. Every other field is a statement about it. */
  target: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  drift_type: string;
  detection_source: string;
  detected_at: string;
}

export interface DriftSummary {
  count: number;
  top?: DriftItem[];
}

/** A target Syndra has not been able to read for itself, and since when. */
export interface UnreconciledTarget {
  target: string;
  since: string;
  last_seen?: string | null;
  reason?: string;
}

export interface GovernanceSummary {
  pending_requests: Array<{ id: string }>;
  expiring_grants: ExpiringGrant[];
  cleanup_hints: string[];
  pending_propagation: PendingPropagationSummary;
  drift: DriftSummary;
  unreconciled_targets: UnreconciledTarget[];
  /** Differences a reconciliation found and was not entitled to resolve: a
   * value the target moved and Syndra did not, a value both moved differently,
   * or an account that is gone. Counted beside drift rather than inside it —
   * drift is access nobody can explain, this is a disagreement everybody can
   * explain and nobody has decided. */
  merge_findings?: number;
}

const KEYS = {
  summary: ["governance", "summary"] as const,
};

/**
 * Pulls the governance dashboard summary — pending request count, grants
 * expiring within the watch window, and least-privilege cleanup hints. Used by
 * both the admin dashboard ("Live operations pulse" sibling) and the audit
 * page watchlist rail.
 */
/**
 * The summary, rebuilt field by field rather than spread.
 *
 * Rebuilding is deliberate: a shape off the network is not a shape this app
 * can trust, and every screen that reads this expects arrays to be arrays. The
 * cost is that a field added to the type and read by a screen arrives as
 * `undefined` until it is named *here*, and nothing complains — TypeScript
 * sees the declared type, the screen sees a silent zero.
 *
 * That is exactly what happened to `merge_findings`: declared on the type,
 * counted inside Today's headline arithmetic, and dropped in transit, so the
 * count had never once been non-zero and the block that would have shown it
 * could never have rendered.
 *
 * Exported so a test can hold it to its own type rather than re-implementing
 * it — a test that copies this function tests the copy.
 */
export function mapGovernanceSummary(data: Partial<GovernanceSummary> | undefined): GovernanceSummary {
  return {
    pending_requests: Array.isArray(data?.pending_requests) ? data.pending_requests : [],
    expiring_grants: Array.isArray(data?.expiring_grants) ? data.expiring_grants : [],
    cleanup_hints: Array.isArray(data?.cleanup_hints) ? data.cleanup_hints : [],
    pending_propagation: data?.pending_propagation ?? { count: 0, zitadel_reachable: true },
    drift: data?.drift ?? { count: 0, top: [] },
    unreconciled_targets: Array.isArray(data?.unreconciled_targets)
      ? data.unreconciled_targets
      : [],
    merge_findings: typeof data?.merge_findings === "number" ? data.merge_findings : 0,
  };
}

export function useGovernanceSummary() {
  return useQuery({
    queryKey: KEYS.summary,
    queryFn: async (): Promise<GovernanceSummary> =>
      mapGovernanceSummary(await request<GovernanceSummary>("/governance/summary")),
  });
}

export const governanceQueryKeys = KEYS;

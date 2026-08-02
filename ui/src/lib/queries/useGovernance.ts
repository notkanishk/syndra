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

export interface GovernanceSummary {
  pending_requests: Array<{ id: string }>;
  expiring_grants: ExpiringGrant[];
  cleanup_hints: string[];
  pending_propagation: PendingPropagationSummary;
  drift: DriftSummary;
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
export function useGovernanceSummary() {
  return useQuery({
    queryKey: KEYS.summary,
    queryFn: async (): Promise<GovernanceSummary> => {
      const data = await request<GovernanceSummary>("/governance/summary");
      return {
        pending_requests: Array.isArray(data?.pending_requests) ? data.pending_requests : [],
        expiring_grants: Array.isArray(data?.expiring_grants) ? data.expiring_grants : [],
        cleanup_hints: Array.isArray(data?.cleanup_hints) ? data.cleanup_hints : [],
        pending_propagation: data?.pending_propagation ?? { count: 0, zitadel_reachable: true },
        drift: data?.drift ?? { count: 0, top: [] },
      };
    },
  });
}

export const governanceQueryKeys = KEYS;

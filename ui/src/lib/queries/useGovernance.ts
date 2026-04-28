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

export interface GovernanceSummary {
  pending_requests: Array<{ id: string }>;
  expiring_grants: ExpiringGrant[];
  cleanup_hints: string[];
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
      };
    },
  });
}

/**
 * Watchlist view is just the watch-window grants from the governance summary.
 * Exposed as a dedicated hook so callers can subscribe to that slice without
 * reading the whole summary; both share the same React Query cache entry.
 */
export function useWatchlist() {
  const summary = useGovernanceSummary();
  return {
    ...summary,
    data: summary.data?.expiring_grants ?? [],
  };
}

export const governanceQueryKeys = KEYS;

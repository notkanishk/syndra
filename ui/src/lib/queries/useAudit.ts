"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface AuditEntry {
  id: string;
  actor_id: string;
  target_id: string;
  action: string;
  resource_id: string;
  created_at: string;
}

export interface AuditFilter {
  /** Page size — backend caps at 200. */
  limit?: number;
}

const KEYS = {
  list: (limit: number) => ["audit", "list", limit] as const,
};

/**
 * Fetches the most recent audit entries. The backend currently exposes only a
 * `?limit=` parameter (max 200) — true cursor pagination is deferred to a
 * later change. The hook accepts `limit` so callers can grow the window via
 * "Load more" without a refetch storm; React Query keeps the cache warm.
 */
export function useAuditEntries(filter: AuditFilter = {}) {
  const limit = filter.limit ?? 50;
  return useQuery({
    queryKey: KEYS.list(limit),
    queryFn: async (): Promise<AuditEntry[]> => {
      const data = await request<unknown>(`/audit?limit=${limit}`);
      return Array.isArray(data) ? (data as AuditEntry[]) : [];
    },
  });
}

export const auditQueryKeys = KEYS;

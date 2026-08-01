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
  /**
   * Narrows to one person's involvement: entries where they are the actor OR
   * the target. Filtered server-side on purpose — client-filtering the global
   * tail would silently truncate, so an account whose last action fell outside
   * the most recent 200 rows would render as "nothing ever happened".
   */
  userId?: string;
}

const KEYS = {
  list: (limit: number) => ["audit", "list", limit] as const,
  forUser: (userId: string, limit: number) => ["audit", "user", userId, limit] as const,
};

/**
 * Fetches the most recent audit entries. The backend currently exposes only a
 * `?limit=` parameter (max 200) — true cursor pagination is deferred to a
 * later change. The hook accepts `limit` so callers can grow the window via
 * "Load more" without a refetch storm; React Query keeps the cache warm.
 */
export function useAuditEntries(filter: AuditFilter = {}) {
  const limit = filter.limit ?? 50;
  const userId = filter.userId ?? "";
  return useQuery({
    queryKey: userId ? KEYS.forUser(userId, limit) : KEYS.list(limit),
    queryFn: async (): Promise<AuditEntry[]> => {
      const scope = userId ? `&user_id=${encodeURIComponent(userId)}` : "";
      const data = await request<unknown>(`/audit?limit=${limit}${scope}`);
      return Array.isArray(data) ? (data as AuditEntry[]) : [];
    },
  });
}

export const auditQueryKeys = KEYS;

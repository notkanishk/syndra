"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

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
  /** Page size — the backend caps a single page at 200. */
  limit?: number;
  /**
   * Narrows to one person's involvement: entries where they are the actor OR
   * the target. Filtered server-side on purpose — client-filtering the global
   * tail would silently truncate, so an account whose last action fell outside
   * the fetched window would render as "nothing ever happened".
   */
  userId?: string;
}

/**
 * Where the next page starts: the (created_at, id) of the last row held.
 *
 * Both halves travel. `created_at` is the transaction timestamp, so a cascade
 * that writes eight audit rows writes eight rows at the identical instant —
 * a timestamp-only cursor would skip the rest of that batch or return it
 * forever.
 */
function cursorParams(last: AuditEntry | undefined): string {
  if (!last) return "";
  return `&before_at=${encodeURIComponent(last.created_at)}&before_id=${encodeURIComponent(last.id)}`;
}

const KEYS = {
  list: (limit: number) => ["audit", "list", limit] as const,
  forUser: (userId: string, limit: number) => ["audit", "user", userId, limit] as const,
  pages: (userId: string, limit: number) => ["audit", "pages", userId, limit] as const,
};

async function fetchPage(limit: number, userId: string, after: AuditEntry | undefined) {
  const scope = userId ? `&user_id=${encodeURIComponent(userId)}` : "";
  const data = await request<unknown>(`/audit?limit=${limit}${scope}${cursorParams(after)}`);
  return Array.isArray(data) ? (data as AuditEntry[]) : [];
}

/**
 * A fixed, most-recent window. For surfaces that want the last N and mean it —
 * the dashboard's eight lines, a person's activity tab — where "load more" is
 * not the question being asked.
 */
export function useAuditEntries(filter: AuditFilter = {}) {
  const limit = filter.limit ?? 50;
  const userId = filter.userId ?? "";
  return useQuery({
    queryKey: userId ? KEYS.forUser(userId, limit) : KEYS.list(limit),
    queryFn: () => fetchPage(limit, userId, undefined),
  });
}

/**
 * The whole log, a page at a time.
 *
 * The audit page used to ask for 200 rows against a backend that capped at 200,
 * with no offset and no cursor: anything older than the most recent 200
 * mutations org-wide was unreachable, and nothing on screen said so. Keyset
 * paging makes the tail walkable; a short page is the end of it.
 */
export function useAuditPages(filter: AuditFilter = {}) {
  const limit = filter.limit ?? 100;
  const userId = filter.userId ?? "";
  return useInfiniteQuery({
    queryKey: KEYS.pages(userId, limit),
    initialPageParam: undefined as AuditEntry | undefined,
    queryFn: ({ pageParam }) => fetchPage(limit, userId, pageParam),
    // A page shorter than the limit is the end. Asking again would return the
    // same nothing, and a "Load more" that does nothing is worse than none.
    getNextPageParam: (lastPage) =>
      lastPage.length < limit ? undefined : lastPage[lastPage.length - 1],
  });
}

export const auditQueryKeys = KEYS;

"use client";

import { useQueries, useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface BundleRow {
  id: string;
  name: string;
  description?: string;
  roles?: string[];
  created_at?: string;
}

export interface BundleRoleRow {
  bundle_id: string;
  zitadel_project_id: string;
  zitadel_role_key: string;
}

const KEYS = {
  list: ["bundles"] as const,
  rolesFor: (id: string) => ["bundles", id, "roles"] as const,
  impactFor: (id: string) => ["bundles", id, "impact"] as const,
};

/** List all bundles. */
export function useBundles() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: async (): Promise<BundleRow[]> => {
      const data = await request<unknown>("/bundles");
      return Array.isArray(data) ? (data as BundleRow[]) : [];
    },
  });
}

/** Fetch the role membership of a single bundle. */
export function useBundleRoles(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: bundleId ? KEYS.rolesFor(bundleId) : ["bundles", "noop", "roles"],
    queryFn: async (): Promise<BundleRoleRow[]> => {
      if (!bundleId) return [];
      const data = await request<unknown>(`/bundles/${bundleId}/roles`);
      return Array.isArray(data) ? (data as BundleRoleRow[]) : [];
    },
    enabled: !!bundleId,
  });
}

/**
 * Fan out one /bundles/{id}/roles query per bundle. Used by /projects which
 * needs the role index for every bundle to compute per-role rollups.
 * Each query is cached independently so navigating away and back hits cache.
 */
export function useBundleRolesByBundle(bundleIds: string[]) {
  const results = useQueries({
    queries: bundleIds.map((id) => ({
      queryKey: KEYS.rolesFor(id),
      queryFn: async (): Promise<BundleRoleRow[]> => {
        const data = await request<unknown>(`/bundles/${id}/roles`);
        return Array.isArray(data) ? (data as BundleRoleRow[]) : [];
      },
    })),
  });

  const byId: Record<string, BundleRoleRow[]> = {};
  bundleIds.forEach((id, idx) => {
    byId[id] = results[idx].data ?? [];
  });
  const allLoaded = results.every((r) => !r.isLoading);
  return { byId, allLoaded };
}

export const bundlesQueryKeys = KEYS;

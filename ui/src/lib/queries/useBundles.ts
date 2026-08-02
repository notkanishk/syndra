"use client";

import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface BundleRow {
  id: string;
  name: string;
  description?: string;
  is_welcome?: boolean;
  roles?: string[];
  confirmation_mode?: "auto" | "manual";
  /** How many people currently hold this bundle. Editing it changes all of them. */
  holder_count?: number;
  /** Highest published version. */
  latest_version?: number;
  /** Holders pinned to something older than the latest version. */
  stale_holders?: number;
  /** Role additions or removals sitting in the working copy, unpublished. */
  unpublished_changes?: number;
  /** Set only when the bundle was read for one person: the version THEY hold. */
  pinned_version?: number;
  created_at?: string;
}

export interface BundleRoleRow {
  bundle_id: string;
  zitadel_project_id: string;
  zitadel_role_key: string;
}

export interface BundleImpactView {
  role_count: number;
  users: Array<{ id: string; name: string }>;
}

export interface CreateBundleInput {
  name: string;
  description: string;
  /** Optional override — omitted falls back to the global default (Task 22). */
  confirmation_mode?: "auto" | "manual";
}

export interface AddBundleRoleInput {
  project_id: string;
  role_key: string;
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

/** Fetch the impact preview for a single bundle. */
export function useBundleImpact(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: bundleId ? KEYS.impactFor(bundleId) : ["bundles", "noop", "impact"],
    queryFn: async (): Promise<BundleImpactView | null> => {
      if (!bundleId) return null;
      return await request<BundleImpactView>(`/bundles/${bundleId}/impact`);
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

/**
 * What a working-copy edit invalidates.
 *
 * The draft query is the one that must not be missed. It backs the unpublished-
 * change strip and the Publish button, so leaving it stale meant an operator
 * added a role, saw the row appear, and had no way to publish it until they
 * reloaded — the edit looked applied and nothing said otherwise.
 *
 * The bundle LIST is included for the same reason: it carries the
 * unpublished-changes marker.
 *
 * `users` is NOT invalidated. An edit reaches nobody, so nobody's access
 * changed; refetching People after one would be claiming otherwise.
 */
function invalidateAfterEdit(qc: ReturnType<typeof useQueryClient>, bundleId: string) {
  qc.invalidateQueries({ queryKey: KEYS.rolesFor(bundleId) });
  qc.invalidateQueries({ queryKey: KEYS.impactFor(bundleId) });
  qc.invalidateQueries({ queryKey: ["bundles", bundleId, "draft"] });
  qc.invalidateQueries({ queryKey: KEYS.list });
}

/** Create a new bundle. Invalidates the list. */
export function useCreateBundle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateBundleInput) => {
      return await request<BundleRow>("/bundles", { method: "POST", body: input });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}

/** Add a (project_id, role_key) pair to a bundle. */
export function useAddBundleRole(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: AddBundleRoleInput) => {
      return await request(`/bundles/${bundleId}/roles`, {
        method: "POST",
        body: input,
      });
    },
    onSuccess: () => invalidateAfterEdit(qc, bundleId),
  });
}

/**
 * Remove a role from a bundle. This is the Advanced half of the split: it
 * changes access for every holder at once, which is why the screen shows the
 * impact breakdown before the click rather than a confirmation after it.
 */
export function useRemoveBundleRole(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ projectId, roleKey }: { projectId: string; roleKey: string }) =>
      request(
        `/bundles/${bundleId}/roles/${encodeURIComponent(projectId)}/${encodeURIComponent(roleKey)}`,
        { method: "DELETE" },
      ),
    onSuccess: () => invalidateAfterEdit(qc, bundleId),
  });
}

/** Set a bundle as the system's welcome bundle (transactional clear-then-set). */
export function useSetWelcomeBundle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (bundleId: string) => {
      return await request(`/bundles/${bundleId}/welcome`, { method: "PUT" });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}

export const bundlesQueryKeys = KEYS;

/**
 * Unassigns a bundle from one person. The bundle itself is untouched — this is
 * the "acting on one person" half of the split, and it is Basic work.
 */
export function useRemoveBundle(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (bundleId: string) =>
      request(`/users/${userId}/bundles/${bundleId}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users", "access", userId] });
      qc.invalidateQueries({ queryKey: ["users", "list"] });
      qc.invalidateQueries({ queryKey: ["bundles"] });
    },
  });
}

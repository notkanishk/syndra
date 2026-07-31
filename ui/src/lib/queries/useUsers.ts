"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

// Shapes mirror the backend `users.go` handler responses. Kept narrow to what
// the Users page renders; widen if more fields are needed.
export interface UserListEntry {
  user: {
    id: string;
    name: string;
    email: string;
    title: string;
    team: string;
    status: string;
    avatar: string;
  };
  bundle_count: number;
  bundle_names: string[];
  effective_role_count: number;
  project_count: number;
  key_projects: string[];
  /**
   * The "needs attention" trio. Each is rendered in the semantic colour it
   * belongs to — expiring amber, open requests accent, unexplained red — and
   * a row with none of them shows a faint dash rather than nothing at all.
   */
  expiring_count: number;
  open_request_count: number;
  unexplained_count: number;
  soonest_expiry?: string | null;
}

export interface AccessRoleReason {
  kind: string;
  description: string;
}

export interface AccessRole {
  role_key: string;
  reasons: AccessRoleReason[];
}

export interface UserAccessProject {
  project_id: string;
  project_name: string;
  source_roles: AccessRole[];
  derived_roles: AccessRole[];
  effective_role_keys: string[];
}

export interface UserAccessView {
  user: UserListEntry["user"];
  bundles: Array<{ id: string; name: string; description: string }>;
  projects: UserAccessProject[];
  cleanup_hints: string[];
}

export interface DirectGrantRow {
  id: string;
  project_id: string;
  role_key: string;
  granted_by: string;
  reason: string;
  expires_at?: string | null;
}

const KEYS = {
  list: (q: string) => ["users", "list", q] as const,
  access: (id: string) => ["users", "access", id] as const,
  grants: (id: string) => ["users", "grants", id] as const,
};

export function useUsers(q: string = "") {
  return useQuery({
    queryKey: KEYS.list(q),
    queryFn: async (): Promise<UserListEntry[]> => {
      const path = q ? `/users?q=${encodeURIComponent(q)}` : "/users";
      const data = await request<unknown>(path);
      return Array.isArray(data) ? (data as UserListEntry[]) : [];
    },
  });
}

export function useUserAccess(userId: string) {
  return useQuery({
    queryKey: KEYS.access(userId),
    queryFn: async (): Promise<UserAccessView | null> => {
      if (!userId) return null;
      return await request<UserAccessView>(`/users/${userId}/access`);
    },
    enabled: !!userId,
  });
}

export function useUserGrants(userId: string) {
  return useQuery({
    queryKey: KEYS.grants(userId),
    queryFn: async (): Promise<DirectGrantRow[]> => {
      if (!userId) return [];
      const data = await request<unknown>(`/users/${userId}/grants`);
      return Array.isArray(data) ? (data as DirectGrantRow[]) : [];
    },
    enabled: !!userId,
  });
}

/**
 * Assigns a bundle to a user. Invalidates the affected user's access view and
 * the global user list (bundle_count is a denormalized field rendered there).
 */
export function useAssignBundle(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (bundleId: string) => {
      return await request(`/users/${userId}/bundles`, {
        method: "POST",
        body: { bundle_id: bundleId },
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.access(userId) });
      qc.invalidateQueries({ queryKey: ["users", "list"] });
    },
  });
}

export interface CreateGrantInput {
  project_id: string;
  role_key: string;
  reason: string;
  duration_days: number;
}

/**
 * Creates a direct grant. The proxy injects `granted_by` for demo sessions and
 * the backend derives it from the JWT subject for OIDC sessions.
 */
export function useCreateGrant(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateGrantInput) => {
      return await request(`/users/${userId}/grants`, { method: "POST", body: input });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.access(userId) });
      qc.invalidateQueries({ queryKey: KEYS.grants(userId) });
      qc.invalidateQueries({ queryKey: ["users", "list"] });
    },
  });
}

export const usersQueryKeys = KEYS;

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * The identity provider's own view, read straight through the proxy.
 *
 * This is deliberately a separate layer from every other query in the app. The
 * rest of Syndra reads its own model of the world; these read what Zitadel
 * actually holds, which is the only way to answer "does it agree with us".
 * Nothing here is cached long — a stale answer to that question is worse than
 * no answer.
 */

export interface UpstreamProject {
  id: string;
  name: string;
  state: string;
}

export interface UpstreamProjectRole {
  key: string;
  displayName: string;
  group: string;
}

export interface UpstreamUser {
  id: string;
  userName: string;
  displayName: string;
  email: string;
  state: string;
}

export interface UpstreamGrant {
  id: string;
  userId: string;
  projectId: string;
  roleKeys: string[];
}

interface Page<T> {
  items: T[];
  total: number;
}

const PAGE_SIZE = 500;

/** Bounds a runaway directory rather than paging forever. */
const AGGREGATE_CAP = 5_000;

const KEYS = {
  projects: ["upstream", "projects"] as const,
  projectRoles: (id: string) => ["upstream", "projects", id, "roles"] as const,
  users: ["upstream", "users"] as const,
  grants: ["upstream", "grants"] as const,
  userGrants: (id: string) => ["upstream", "users", id, "grants"] as const,
};

async function page<T>(path: string): Promise<Page<T>> {
  const raw = await request<Partial<Page<T>>>(path);
  return { items: Array.isArray(raw?.items) ? raw.items : [], total: raw?.total ?? 0 };
}

export function useUpstreamProjects() {
  return useQuery({
    queryKey: KEYS.projects,
    queryFn: () => page<UpstreamProject>(`/zitadel/projects?limit=${PAGE_SIZE}`),
    retry: false,
  });
}

export function useUpstreamProjectRoles(projectId: string | null) {
  return useQuery({
    queryKey: projectId ? KEYS.projectRoles(projectId) : ["upstream", "projects", "none", "roles"],
    queryFn: () =>
      page<UpstreamProjectRole>(`/zitadel/projects/${projectId}/roles?limit=${PAGE_SIZE}`),
    enabled: Boolean(projectId),
    retry: false,
  });
}

export function useUpstreamUsers() {
  return useQuery({
    queryKey: KEYS.users,
    queryFn: () => page<UpstreamUser>(`/zitadel/users?limit=${PAGE_SIZE}`),
    retry: false,
  });
}

export function useUpstreamUserGrants(userId: string | null) {
  return useQuery({
    queryKey: userId ? KEYS.userGrants(userId) : ["upstream", "users", "none", "grants"],
    queryFn: () => page<UpstreamGrant>(`/zitadel/users/${userId}/grants?limit=${PAGE_SIZE}`),
    enabled: Boolean(userId),
    retry: false,
  });
}

/**
 * Every grant the provider holds, paged to exhaustion or to the safety cap.
 * `truncated` means the ledger on screen is partial — the UI must say so, or
 * somebody will read an absence as proof.
 */
export function useUpstreamGrants() {
  return useQuery({
    queryKey: KEYS.grants,
    queryFn: async (): Promise<{ items: UpstreamGrant[]; total: number; truncated: boolean }> => {
      const all: UpstreamGrant[] = [];
      let offset = 0;
      let total = 0;
      let truncated = false;
      for (;;) {
        const result = await page<UpstreamGrant>(
          `/zitadel/grants?limit=${PAGE_SIZE}&offset=${offset}`,
        );
        total = result.total || all.length + result.items.length;
        all.push(...result.items);
        if (result.items.length === 0 || all.length >= total) break;
        if (all.length >= AGGREGATE_CAP) {
          truncated = true;
          break;
        }
        offset += result.items.length;
      }
      return { items: all, total, truncated };
    },
    retry: false,
  });
}

// ---------------------------------------------------------------------------
// Writes. These bypass Syndra's ledger and outbox entirely — see the warning
// the Identity provider page renders above them. Kept here rather than beside
// the ordinary mutations so nothing reaches for one by accident.
// ---------------------------------------------------------------------------

function useUpstreamMutation<V>(fn: (value: V) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["upstream"] });
      // Anything written directly upstream will surface as drift on the next
      // sweep; invalidating here means the operator sees that immediately
      // rather than wondering why the number moved tomorrow.
      qc.invalidateQueries({ queryKey: ["drift"] });
    },
  });
}

export const useUpstreamAssignGrant = () =>
  useUpstreamMutation(({ userId, projectId, roleKeys }: { userId: string; projectId: string; roleKeys: string[] }) =>
    request(`/zitadel/users/${userId}/grants`, {
      method: "POST",
      body: { projectId, roleKeys },
    }),
  );

export const useUpstreamUpdateGrant = () =>
  useUpstreamMutation(({ userId, grantId, roleKeys }: { userId: string; grantId: string; roleKeys: string[] }) =>
    request(`/zitadel/users/${userId}/grants/${grantId}`, {
      method: "PUT",
      body: { roleKeys },
    }),
  );

export const useUpstreamRemoveGrant = () =>
  useUpstreamMutation(({ userId, grantId }: { userId: string; grantId: string }) =>
    request(`/zitadel/users/${userId}/grants/${grantId}`, { method: "DELETE" }),
  );

export const useUpstreamCreateRole = () =>
  useUpstreamMutation(
    ({ projectId, key, displayName, group }: { projectId: string; key: string; displayName: string; group: string }) =>
      request(`/zitadel/projects/${projectId}/roles`, {
        method: "POST",
        body: { roleKey: key, displayName, group },
      }),
  );

export const useUpstreamUpdateRole = () =>
  useUpstreamMutation(
    ({ projectId, key, displayName, group }: { projectId: string; key: string; displayName: string; group: string }) =>
      request(`/zitadel/projects/${projectId}/roles/${encodeURIComponent(key)}`, {
        method: "PUT",
        body: { displayName, group },
      }),
  );

export const useUpstreamDeleteRole = () =>
  useUpstreamMutation(({ projectId, key }: { projectId: string; key: string }) =>
    request(`/zitadel/projects/${projectId}/roles/${encodeURIComponent(key)}`, {
      method: "DELETE",
    }),
  );

export const upstreamQueryKeys = KEYS;

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { RetryState } from "@/lib/retry";
import type { ActPlan, RolePlans } from "@/lib/usePlanConsent";

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
  /**
   * When this page was actually observed, and whether that observation was
   * whole — set only by the two grants endpoints, which read through
   * `internal/observe` rather than listing Zitadel directly. Absent on every
   * other paginated endpoint here.
   */
  observedAt?: string;
  complete?: boolean;
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
  const raw = await request<Partial<Page<T>> & { observed_at?: string }>(path);
  return {
    items: Array.isArray(raw?.items) ? raw.items : [],
    total: raw?.total ?? 0,
    observedAt: raw?.observed_at,
    complete: raw?.complete,
  };
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

/**
 * What revoking one grant's roles would revoke and keep, from each role's
 * plan on the server (GET …/grants/{grantId}/removal, re-audit R18, task
 * 1.9): the count a revoke asks consent for is never worked out in the
 * browser, and `plans` are what the write carries back. `roles` narrows it to
 * a replace's left-out roles; none means every role on the grant.
 */
export function useGrantRemovalPreview(userId: string, grantId: string, roles?: string[], enabled = true, alongside: string[] = []) {
  // `alongside` are the roles a replace gives first: the removals are planned beside them.
  const only = (roles ? `?roles=${encodeURIComponent(roles.join(","))}` : "") + (alongside.length ? `${roles ? "&" : "?"}with=${encodeURIComponent(alongside.join(","))}` : "");
  return useQuery({
    queryKey: ["upstream", "users", userId, "grants", grantId, "removal", only],
    queryFn: () =>
      request<ZitadelGrantChange>(
        `/zitadel/users/${encodeURIComponent(userId)}/grants/${encodeURIComponent(grantId)}/removal${only}`,
      ),
    enabled: enabled && Boolean(userId && grantId) && (!roles || roles.length > 0),
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
    queryFn: async (): Promise<{
      items: UpstreamGrant[];
      total: number;
      truncated: boolean;
      observedAt?: string;
    }> => {
      const all: UpstreamGrant[] = [];
      let offset = 0;
      let total = 0;
      let truncated = false;
      let observedAt: string | undefined;
      for (;;) {
        const result = await page<UpstreamGrant>(
          `/zitadel/grants?limit=${PAGE_SIZE}&offset=${offset}`,
        );
        total = result.total || all.length + result.items.length;
        all.push(...result.items);
        observedAt = result.observedAt ?? observedAt;
        // A page whose own observation stopped short (Zitadel didn't answer,
        // or the safety cap) makes the WHOLE aggregate incomplete — the same
        // meaning `truncated` already carries for the aggregate cap below.
        if (result.complete === false) truncated = true;
        if (result.items.length === 0 || all.length >= total) break;
        if (all.length >= AGGREGATE_CAP) {
          truncated = true;
          break;
        }
        offset += result.items.length;
      }
      return { items: all, total, truncated, observedAt };
    },
    retry: false,
  });
}

// ---------------------------------------------------------------------------
// Writes. Each goes through the same pipe as its everyday screen: a person's
// roles as a person's page changes them, a role as Access › Roles makes it.
// Kept here rather than beside the ordinary mutations so nothing reaches for
// one by accident.
// ---------------------------------------------------------------------------

/**
 * What a change on the Zitadel › People screen did, role by role ("project/role").
 * Retained roles stay because a bundle or a rule still gives them.
 */
export interface ZitadelGrantChange {
  given_roles?: string[];
  revoked_roles?: string[];
  retained_roles?: string[];
  /** The least delivered role's state: applied, pending (incl. waiting), failed, or unchanged (all kept). */
  status?: "applied" | "pending" | "failed" | "unchanged";
  wait_reason?: string;
  /** With halted:retrying: when Syndra sends what Zitadel didn't take again (re-audit R3). */
  retry?: RetryState;
  /** Each role's own outcome (W8). */
  roles?: ZitadelRoleChange[];
  /** A removal preview's plan per role key, which the write carries back. */
  plans?: Record<string, ActPlan>;
}

/** One role's part of a change, after delivery. */
export interface ZitadelRoleChange {
  /** "project/role". */
  role: string;
  change: "given" | "revoked" | "kept";
  /** applied: in Zitadel now; waiting: held for you to send; pending: not taken yet; kept: nothing sent. */
  status: "applied" | "waiting" | "pending" | "failed" | "kept";
  /** Why a failed role isn't in Zitadel: `refused:<cause>` (lib/wait-reason.ts refusalCause). */
  reason?: string;
}

function useUpstreamMutation<V, R = unknown>(fn: (value: V) => Promise<R>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    // Settled, not only succeeded: a write whose outcome is unknown (the
    // answer was lost, C6) may have changed Zitadel, so every reader re-reads.
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["upstream"] });
      qc.invalidateQueries({ queryKey: ["drift"] });
      // A role created or renamed here changes the names every screen resolves.
      qc.invalidateQueries({ queryKey: ["name-catalog"] });
      // The same records the everyday screens read: a person's access, a
      // role's holders and the catalogue, the queue of changes to send.
      qc.invalidateQueries({ queryKey: ["users"] });
      qc.invalidateQueries({ queryKey: ["roles"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
      qc.invalidateQueries({ queryKey: ["propagations"] });
    },
  });
}

// Each role is one planned act, as on a person's page (task 1.9): the write
// carries the plan each role was shown (`plans`, by role key), and is sent
// as those plans decided, straight away unless its role waits.

export const useUpstreamAssignGrant = () =>
  useUpstreamMutation(
    ({ userId, projectId, roleKeys, plans }: { userId: string; projectId: string; roleKeys: string[]; plans?: RolePlans }) =>
      request<ZitadelGrantChange>(`/zitadel/users/${userId}/grants`, {
        method: "POST",
        body: { projectId, roleKeys, plans },
      }),
  );

export const useUpstreamUpdateGrant = () =>
  useUpstreamMutation(
    ({ userId, grantId, roleKeys, plans }: { userId: string; grantId: string; roleKeys: string[]; plans?: RolePlans }) =>
      request<ZitadelGrantChange>(`/zitadel/users/${userId}/grants/${grantId}`, {
        method: "PUT",
        body: { roleKeys, plans },
      }),
  );

export const useUpstreamRemoveGrant = () =>
  useUpstreamMutation(({ userId, grantId, plans }: { userId: string; grantId: string; plans?: RolePlans }) =>
    request<ZitadelGrantChange>(`/zitadel/users/${userId}/grants/${grantId}`, { method: "DELETE", body: { plans } }),
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

/**
 * Refused with 409 ROLE_HELD while anybody holds the role in Zitadel; the
 * refusal's details carry `count` and `holders`, and sending that same count
 * back as `confirmHolders` is the confirmation.
 */
export const useUpstreamDeleteRole = () =>
  useUpstreamMutation(({ projectId, key, confirmHolders }: { projectId: string; key: string; confirmHolders?: number }) =>
    request(
      `/zitadel/projects/${projectId}/roles/${encodeURIComponent(key)}` +
        (confirmHolders ? `?confirm_holders=${confirmHolders}` : ""),
      { method: "DELETE" },
    ),
  );

export const upstreamQueryKeys = KEYS;

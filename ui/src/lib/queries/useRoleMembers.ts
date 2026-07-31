"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type { RoleReason } from "@/components/access/AccessSource";
import { request } from "@/lib/api-client";

/** One holder of a (project, role) pair, with the sources that produced it. */
export interface RoleMember {
  user: { id: string; name: string; email: string; title: string; team: string };
  reasons: RoleReason[];
  since?: string;
  expires?: string;
  /** Present only for a direct source — the id the removal endpoint takes. */
  grant_id?: string;
}

export interface RoleMembersView {
  project_id: string;
  project_name: string;
  role_key: string;
  display_name?: string;
  description?: string;
  group?: string;
  cloned_from?: string;
  members: RoleMember[];
  direct_count: number;
  bundle_count: number;
  automatic_count: number;
}

export function useRoleMembers(projectId: string, roleKey: string) {
  return useQuery({
    queryKey: ["roles", "members", projectId, roleKey],
    queryFn: () =>
      request<RoleMembersView>(
        `/projects/${encodeURIComponent(projectId)}/roles/${encodeURIComponent(roleKey)}/members`,
      ),
    enabled: Boolean(projectId && roleKey),
  });
}

/**
 * Removes one direct grant: the MkAuth ledger row goes away and the Zitadel
 * revoke is queued in the same transaction.
 *
 * Deliberately not the Zitadel-side grant delete — that removes a different
 * object, leaves this row behind, and the next cache compile puts the access
 * straight back.
 */
export function useRemoveDirectGrant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, grantId }: { userId: string; grantId: string }) =>
      request(`/users/${userId}/grants/${grantId}?apply=true`, { method: "DELETE" }),
    onSuccess: (_data, { userId }) => {
      qc.invalidateQueries({ queryKey: ["users", "access", userId] });
      qc.invalidateQueries({ queryKey: ["users", "grants", userId] });
      qc.invalidateQueries({ queryKey: ["roles", "members"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

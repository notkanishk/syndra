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
  /**
   * What this role confers and this person does not have.
   *
   * §6: the carve-out has to be visible everywhere the role appears, and this
   * list is where an operator picks who to act on. Without it the screen says
   * "these people hold this role" and means "most of them do".
   */
  withheld?: Array<{
    target: string;
    field: string;
    value: string;
    reason: string;
    actor_id: string;
    allowance_id: string;
  }>;
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
  /** How many holders have something withheld. Counted apart from the source
   *  pills below: a carve-out is orthogonal to how somebody came to hold it. */
  withheld_count: number;
  /**
   * The carve-out read failed, so this list does not know whether anybody holds
   * the role with something taken away.
   *
   * Rendered, never swallowed: a zero count with this set means "unknown", and
   * a zero count without it means "none". Those are different sentences and the
   * page has to say which one it is looking at.
   */
  withheld_unavailable?: boolean;
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
 * Removes one direct grant: the Syndra ledger row goes away and the Zitadel
 * revoke is queued in the same transaction.
 *
 * Deliberately not the Zitadel-side grant delete — that removes a different
 * object, leaves this row behind, and the next cache compile puts the access
 * straight back.
 */
/**
 * What the backend says the removal actually did.
 *
 * Removing one grant is not the same as removing the access: a role this
 * person also holds through a bundle or a rule survives, and the closure diff
 * that decides which is which is computed server-side. The UI must not have a
 * second opinion about access, so the dialog states these rather than
 * inferring anything.
 */
export interface DirectGrantRemoval {
  /** Roles that genuinely went away upstream. */
  revoked_roles?: string[];
  /** Roles kept, because something else still supplies them. */
  retained_roles?: string[];
}

export function useRemoveDirectGrant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, grantId }: { userId: string; grantId: string }) =>
      request<DirectGrantRemoval>(`/users/${userId}/grants/${grantId}?apply=true`, {
        method: "DELETE",
      }),
    onSuccess: (_data, { userId }) => {
      qc.invalidateQueries({ queryKey: ["users", "access", userId] });
      qc.invalidateQueries({ queryKey: ["users", "grants", userId] });
      qc.invalidateQueries({ queryKey: ["roles", "members"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

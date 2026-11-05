"use client";

import { settleIndicators } from "@/lib/queries/useIndicators";
import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { ObservationBasis } from "@/lib/queries/useRoles";
import type { CascadeOutcome } from "@/lib/types";
import { sortByName } from "@/lib/sort";
import { invalidateAfter } from "./invalidation";
import type { RetryState } from "@/lib/retry";

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
    /** What Zitadel says the account is. Absent reads as a person. */
    kind?: "human" | "machine";
    /** Effective: machine or marked (A-07 §26). */
    service_account?: boolean;
    joined?: string;
  };
  bundle_count: number;
  bundle_names: string[];
  /** Bundle name → the version THIS person is pinned to. */
  bundle_versions?: Record<string, number>;
  effective_role_count: number;
  project_count: number;
  /** Project display names — what the row and the filter dropdown show. */
  key_projects: string[];
  /** The same projects by id — what a shareable filter URL carries. */
  key_project_ids: string[];
  /**
   * The "needs attention" trio. Each is rendered in the semantic colour it
   * belongs to — expiring amber, open requests accent, unexplained red — and
   * a row with none of them shows a faint dash rather than nothing at all.
   */
  expiring_count: number;
  open_request_count: number;
  unexplained_count: number;
  /** Changes Zitadel accepted for this person that no read has shown back yet. Absent reads as none. */
  unseen_count?: number;
  soonest_expiry?: string | null;
  /**
   * What Zitadel shows this person holding right now — not Syndra's record of
   * what was granted. Absent, not zero, when the org has never been read.
   */
  observed_role_count?: number;
  observed_project_count?: number;
  observation?: ObservationBasis;
}

export interface AccessRoleReason {
  kind: string;
  description: string;
  bundle_id?: string;
  bundle_name?: string;
  /** The included bundle a bundle role comes through: "through {bundle}, from {via}". */
  via_bundle_id?: string;
  via_bundle_name?: string;
  trigger_project?: string;
  trigger_role?: string;
  /** Recorded, not yet sent to Zitadel. */
  queued?: boolean;
  /** When it was queued (RFC3339; the earliest when several rows wait on this source). Only with `queued`. */
  queued_at?: string;
}

export interface AccessRole {
  role_key: string;
  reasons: AccessRoleReason[];
  /** When it ends, from every source (a rule role with its source); absent: never. */
  ends?: string | null;
}

export interface UserAccessProject {
  project_id: string;
  project_name: string;
  /**
   * False when the directory couldn't name this project and `project_name`
   * is the raw id standing in for one — the id an operator needs to search,
   * not a name. Render it labelled, not as the heading.
   */
  project_name_resolved: boolean;
  source_roles: AccessRole[];
  derived_roles: AccessRole[];
  effective_role_keys: string[];
  /**
   * What Zitadel shows this person holding on this project — the roles they
   * can actually use. The three lists above are Syndra's records: what was
   * decided, and by whom. This is what is true.
   *
   * null — not empty — while nothing has been observed, or while the read
   * that happened was too incomplete to conclude an absence. An empty array is
   * a checked absence and may be rendered as one; null may not.
   */
  observed_role_keys: string[] | null;
}

export interface UserAccessView {
  user: UserListEntry["user"];
  bundles: Array<{
    id: string;
    name: string;
    description: string;
    /** The version of this bundle THEY hold. */
    pinned_version?: number;
    /** The bundle's highest published version, so a gap is visible on the row. */
    latest_version?: number;
  }>;
  projects: UserAccessProject[];
  /**
   * The second layer of the access model: what has been decided about this
   * person on top of what their roles give them. Present whether or not any is
   * in force — a suspension that ended is part of the answer to "what has been
   * decided about this person".
   *
   * Rendered, not merely carried. Without it a role-holder list reads as full
   * access while a subtractive allowance is withholding the entitlement that
   * role maps to, which is precisely the trap §6 exists to close.
   */
  allowances: AllowanceBand[];
  cleanup_hints: string[];
  /**
   * How many roles Zitadel shows this person holding, across every project.
   * The headline number on any screen answering "what can I use" — absent,
   * never zero, while nothing has been observed.
   */
  observed_role_count?: number;
  /** When that was read, whether it succeeded, and whether it saw everything. */
  observation: ObservationBasis;
}

export interface AllowanceBand {
  id: string;
  target: string;
  field: string;
  value: string;
  direction: string;
  actor_id: string;
  reason: string;
  /** Derived at read time, so neither can go stale while the date passes. */
  in_force: boolean;
  review_due: boolean;
  created_at: string;
  /** When and how it stopped applying. Empty while it still applies. */
  ended?: string;
  ended_by?: string;
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
    // A search is a new read, but the rows on screen are not a loading list: they stay
    // until the answer lands, and the ones it drops leave (A-05 §2.10).
    placeholderData: keepPreviousData,
  });
}

const fetchUserAccess = async (userId: string): Promise<UserAccessView | null> => {
  if (!userId) return null;
  return await request<UserAccessView>(`/users/${userId}/access`);
};

/** Warms a person's access read (A-19 P9). The same key and read the page uses, so its first frame is complete. */
export function prefetchUserAccess(qc: QueryClient, userId: string) {
  return qc.prefetchQuery({ queryKey: KEYS.access(userId), queryFn: () => fetchUserAccess(userId) });
}

export function useUserAccess(userId: string) {
  return useQuery({
    queryKey: KEYS.access(userId),
    queryFn: () => fetchUserAccess(userId),
    enabled: !!userId,
    // Bundles and projects A to Z (RULES.md); the roles inside go by key.
    select: (view) =>
      view && {
        ...view,
        bundles: sortByName(view.bundles ?? [], (b) => b.name),
        projects: sortByName(view.projects ?? [], (p) => p.project_name, (p) => p.project_id),
      },
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
      return await request<CascadeOutcome>(`/users/${userId}/bundles`, {
        method: "POST",
        body: { bundle_id: bundleId },
      });
    },
    onSuccess: () => invalidateAfter(qc, "access"),
  });
}

export interface CreateGrantInput {
  project_id: string;
  role_key: string;
  reason: string;
  duration_days: number;
  /**
   * Moves the end of a grant they already have, to a later one. Without it,
   * giving a role they already have is refused (owner, 15 Oct).
   */
  extend?: boolean;
}

/** The outbox handle the direct-grant endpoint returns. `status` is "pending"
 * unless the caller opts into inline apply, which this hook never does — so
 * the caller must not assume "applied" from a bare 202. */
export interface CreateGrantResult {
  outbox_id: string;
  status: string;
  /** Why the grant isn't in Zitadel yet: role_waits, halted:<reason> or manual. */
  wait_reason?: string;
  /** With halted:retrying: when Syndra sends it again, and which try. */
  retry?: RetryState;
  /** The rows the automatic rules queued with it. */
  rule_outbox_ids?: string[];
  /** How many of those wait for you to send because their role waits (re-audit R12). */
  rule_waiting?: number;
}

/**
 * Creates a direct grant. The proxy injects `granted_by` for demo sessions and
 * the backend derives it from the JWT subject for OIDC sessions.
 */
export function useCreateGrant(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateGrantInput) => {
      return await request<CreateGrantResult>(`/users/${userId}/grants`, { method: "POST", body: input });
    },
    onSuccess: (made) => {
      // Given but not yet sent: it is now one more thing waiting for the operator.
      if (made?.status !== "applied" || made?.wait_reason) settleIndicators(qc, { pending_propagation: 1 });
      invalidateAfter(qc, "access");
    },
  });
}

export const usersQueryKeys = KEYS;

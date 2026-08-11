"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * The add-on roster and the reads hanging off it.
 *
 * The roster is deployment configuration. It is fetched once and cached for a
 * long time on purpose: a target appearing or disappearing is a deploy, not an
 * event, and refetching it on a timer would make navigation flicker in response
 * to a poll.
 *
 * Everything else here is runtime state and is fetched per page.
 */

export interface TargetOperation {
  id: string;
  scope: string;
  confirm: boolean;
  available: boolean;
  unavailable_reason?: string;
  secret_params?: string[];
}

export interface TargetSummary {
  target: string;
  registered: boolean;
  auth_mode: string;
  /** A manifest has been read and understood. Registration alone offers nothing. */
  callable: boolean;
  operations: TargetOperation[];
  manifest_fetched_at?: string;
  circuit_open: boolean;
  last_error?: string;
}

export function useTargets() {
  return useQuery({
    queryKey: ["targets"],
    queryFn: () => request<{ targets: TargetSummary[] }>("/targets"),
    // Deployment configuration: stale only when the deployment changes, which
    // is a restart.
    staleTime: 10 * 60_000,
    select: (data) => data.targets ?? [],
  });
}

export interface TargetHealth {
  reachable: boolean;
  product?: string;
  product_version?: string;
  version_tested?: boolean;
  version_note?: string;
  circuit_open?: boolean;
  lifecycle?: string;
  lifecycle_note?: string;
  in_flight?: number;
  drained?: boolean;
  log_head?: string;
  log_records?: number;
  snapshot_taken_at?: string;
  last_read_at?: string;
  key_expires_at?: string;
  detail?: string;
  /**
   * The backend's memory of this target's mutation log, when it is carrying a
   * finding. Two authorities in one payload on purpose: the add-on reports its
   * own chain head, and the add-on cannot be the source of truth about whether
   * its own record has been edited.
   */
  log_anchor?: LogAnchor;
}

export interface LogAnchor {
  target: string;
  /** Where the anchor stopped. It deliberately does not advance past a finding. */
  head: string;
  records: number;
  anchored_at: string;
  /** `records_decreased` or `head_rewritten`. Absent on a healthy anchor. */
  violation_reason?: string;
  violation_head?: string;
  violation_records?: number;
  violation_at?: string;
}

export function useTargetHealth(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "health"],
    queryFn: () => request<TargetHealth>(`/targets/${target}/health`),
    enabled: Boolean(target),
    refetchInterval: 30_000,
  });
}

/**
 * Clearing a tamper finding by adopting the head that produced it.
 *
 * Not "dismiss". There is no state where the finding is acknowledged and the
 * anchor is still frozen — that state detects nothing and reads as handled — so
 * resolving means re-baselining, and the copy says so.
 *
 * The cited head is what makes it an operator decision: re-baselining to
 * whatever the target reports at the moment of the click would adopt a chain
 * that changed again while the dialog was open, which is the event the anchor
 * exists to notice.
 */
export function useResolveLogFinding(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { head: string; note: string }) =>
      request<LogAnchor>(`/targets/${target}/log-anchor/resolve`, {
        method: "POST",
        body: { head: input.head, note: input.note, confirmed: true },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "health"] });
    },
  });
}

export interface UnmanagedAccount {
  username: string;
  uid?: number;
}

/**
 * What an adoption came back as.
 *
 * Three outcomes, and only one of them is "adopted". The endpoint used to
 * answer 200 for all three — a target that refused and a target that never
 * answered both produced "The account is now bound to that person" — on the one
 * action in the product that hands one person's data to another and has no undo.
 */
export interface AdoptionResult {
  status: "adopted" | "unconfirmed";
  detail?: string;
  outcome?: string;
  warning?: string;
}

/** One account Syndra DOES manage, and who it belongs to. */
export interface BoundAccount {
  target: string;
  subject_id: string;
  username: string;
  account_uid?: number;
  bound_by: string;
  bound_at: string;
  last_seen_at: string;
}

export interface TargetInventory {
  target: string;
  bound: number;
  /**
   * The accounts Syndra manages. The other half of "whose accounts are on this
   * target" — the inventory answered only which ones it does NOT manage, and
   * those are not the ones an operator acts on.
   */
  accounts?: BoundAccount[];
  unmanaged: UnmanagedAccount[];
  read_at?: string;
  current: boolean;
  truncated?: boolean;
  halted?: boolean;
  reason?: string;
}

export function useTargetInventory(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "inventory"],
    queryFn: () => request<TargetInventory>(`/targets/${target}/inventory`),
    enabled: Boolean(target),
  });
}

/**
 * Adopting an unmanaged account.
 *
 * Confirmed at the call, never only in a dialog: the backend refuses without it,
 * and a confirmation only the frontend enforces is a suggestion. Adopting the
 * wrong account hands a member somebody else's home directory, their shares and
 * their group memberships, and the next convergence makes that look intended.
 */
export function useAdoptAccount(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { username: string; subjectId: string }) =>
      request<AdoptionResult>(
        `/targets/${target}/inventory/${encodeURIComponent(input.username)}/adopt`,
        { method: "POST", body: { subject_id: input.subjectId, confirmed: true } },
      ),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "inventory"] });
      // The binding count on this page comes from the same read. Leaving it
      // stale shows an account that has just left the unmanaged list without
      // appearing anywhere else, which reads as a row that vanished.
      client.invalidateQueries({ queryKey: ["targets", target, "health"] });
    },
  });
}

/** Stopping or resuming an add-on's writing, without a redeploy. */
export function useSetLifecycle(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { state: string; reason: string }) =>
      request<TargetHealth>(`/targets/${target}/lifecycle`, { method: "POST", body: input }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "health"] });
    },
  });
}

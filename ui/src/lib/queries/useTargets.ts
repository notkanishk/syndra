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
  /** Whether that secret still loads — read now, not at start-up. */
  transport_status?: string;
  transport_error?: string;
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
  /**
   * Which of three states the credential is in. `set` carries a date above;
   * `none` is a key issued deliberately without an expiry; `unrecorded` is a
   * date nobody told Syndra — the one worth acting on, because a key CAN expire
   * without Syndra knowing and a silently expired key looks like an outage.
   */
  key_expiry?: "set" | "none" | "unrecorded";
  /** SMB shares with auditing switched off. Activity reports for a member on
   * one of these can only ever come back empty. */
  unaudited_shares?: string[];
  /** Whether the share list could be read at all — "nothing is unaudited" and
   * "could not look" must not render as the same thing. */
  shares_readable?: boolean;
  detail?: string;
  /**
   * The backend's memory of this target's mutation log, when it is carrying a
   * finding. Two authorities in one payload on purpose: the add-on reports its
   * own chain head, and the add-on cannot be the source of truth about whether
   * its own record has been edited.
   */
  log_anchor?: LogAnchor;
  /**
   * Where two of Syndra's own records disagree about who owns an account.
   *
   * Not the add-on's opinion and not drift: both stores were written by this
   * system and neither is authoritative, which is why it needs a person rather
   * than a reconcile.
   */
  binding_conflicts?: BindingConflict[];
}

export interface BindingConflict {
  id: string;
  target: string;
  username: string;
  account_uid?: number;
  /** Whose change landed on the account. */
  converged_subject_id: string;
  /** Who Syndra's own binding says owns it. Neither is authoritative. */
  bound_subject_id: string;
  outbox_id: string;
  detected_at: string;
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

/**
 * Deciding who owns a disputed account.
 *
 * It moves the account between people in Syndra's records and touches nothing
 * on the target — which is the distinction the copy has to carry, because an
 * operator who reads it as "fixed" will not converge.
 */
export function useResolveBindingConflict(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { id: string; owner: string; note: string }) =>
      request<{ resolved: boolean; detail: string }>(
        `/targets/${target}/binding-conflicts/${input.id}/resolve`,
        { method: "POST", body: { owner: input.owner, note: input.note, confirmed: true } },
      ),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "health"] });
      client.invalidateQueries({ queryKey: ["targets", target, "inventory"] });
    },
  });
}

export interface UnmanagedAccount {
  username: string;
  uid?: number;
  /**
   * The add-on's own service account on the target.
   *
   * A real, genuinely unmanaged account — and the one whose deletion removes
   * Syndra's access to the target altogether. The add-on refuses to adopt or
   * purge it whatever any caller asks; this is here so the row says so instead
   * of offering an action that will be refused.
   */
  self?: boolean;
  /** Whether the account has a usable credential yet. */
  password_set?: boolean;
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

/**
 * Reconciling one target now, rather than waiting for the six-hour sweep.
 *
 * It queues and does not apply, which is what makes it safe to press twice.
 */
export function useReconcileTarget(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => request<TargetReconcileResult>(`/targets/${target}/reconcile`, { method: "POST" }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target] });
      client.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

/**
 * Letting a binding go — Syndra stops managing the account, and the account
 * itself is not touched.
 *
 * The other resolution the reconciliation names for a binding whose account is
 * gone. Confirmed at the call for the same reason adoption is: the backend
 * refuses without it, and a confirmation only this file knows about is a
 * suggestion.
 *
 * Reversible in the sense that matters — the account reappears in the unmanaged
 * inventory and can be adopted again — which is why it is a press and not a
 * typed name. The unrecoverable neighbour is `account.purge`, and the two must
 * not feel the same.
 */
export function useReleaseBinding(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (subjectId: string) =>
      request<ReleaseResult>(
        `/targets/${target}/bindings/${encodeURIComponent(subjectId)}/release`,
        { method: "POST", body: { confirmed: true } },
      ),
    onSuccess: () => {
      // The roster, the health card and the inventory all count bindings, and
      // the released account joins the unmanaged list. Leaving any of them
      // stale shows a row that has left one list without arriving in another.
      client.invalidateQueries({ queryKey: ["targets", target] });
      client.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

export interface ReleaseResult {
  status: string;
  operation?: string;
  detail?: string;
  /** Set when the add-on let go and Syndra's own copy did not. Pressing again
   * repairs it, and the copy says so. */
  warning?: string;
}

/**
 * The differences a reconciliation found and was not entitled to resolve.
 *
 * Three kinds, and the surface must keep them apart: the target moved and
 * Syndra did not, both moved differently, or the account is gone. They read as
 * one "out of step" only to somebody who does not have to act on them.
 */
export interface MergeFinding {
  id: string;
  target: string;
  subject_id: string;
  /** Empty for an account-level finding — `deleted_upstream` is about the
   * account existing, not about any value. */
  field?: string;
  outcome: "theirs_only" | "conflict" | "deleted_upstream";
  /** What the target last reported, what Syndra wants, and what the target has
   * now. All three, because "what was it before" is the question asked first. */
  base?: unknown;
  ours?: unknown;
  theirs?: unknown;
  detected_at: string;
  last_seen_at: string;
}

export function useMergeFindings(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "merge-findings"],
    queryFn: async () =>
      (await request<{ findings: MergeFinding[] }>(`/targets/${target}/merge-findings`)).findings ?? [],
    enabled: Boolean(target),
  });
}

export interface ResolveFindingInput {
  id: string;
  resolution: "keep_ours" | "take_theirs" | "reprovisioned" | "unbound";
  reason: string;
  expires_at?: string;
  review_date?: string;
}

/**
 * Carrying out the operator's decision.
 *
 * The backend performs the resolution and only then closes the finding, and it
 * refuses the adoptions that have nowhere to live — a group value belongs to a
 * role mapping that reaches every holder of that role. Those refusals arrive
 * here as errors with the policy named in them, and they are rendered rather
 * than swallowed: the alternative is a button that silently does nothing.
 */
export function useResolveMergeFinding(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: ResolveFindingInput) =>
      request<{ resolved: boolean }>(
        `/targets/${target}/merge-findings/${encodeURIComponent(input.id)}/resolve`,
        {
          method: "POST",
          body: {
            resolution: input.resolution,
            reason: input.reason,
            ...(input.expires_at ? { expires_at: input.expires_at } : {}),
            ...(input.review_date ? { review_date: input.review_date } : {}),
          },
        },
      ),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target] });
      client.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

export interface StaleBinding {
  subject_id: string;
  username: string;
  uid?: number;
}

export interface TargetReconcileResult {
  target: string;
  bound: number;
  queued: number;
  current: boolean;
  truncated?: boolean;
  reason?: string;
  unmanaged?: Array<{ username: string; uid: number }>;
  /** Bindings whose account is no longer on the target. Never converged — the
   * plan for one says "create", and acting on it would recreate an account
   * somebody deleted. */
  stale?: StaleBinding[];
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

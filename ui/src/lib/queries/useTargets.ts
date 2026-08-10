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
}

export function useTargetHealth(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "health"],
    queryFn: () => request<TargetHealth>(`/targets/${target}/health`),
    enabled: Boolean(target),
    refetchInterval: 30_000,
  });
}

export interface UnmanagedAccount {
  username: string;
  uid?: number;
}

export interface TargetInventory {
  target: string;
  bound: number;
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
      request<{ status: string; detail?: string }>(
        `/targets/${target}/inventory/${encodeURIComponent(input.username)}/adopt`,
        { method: "POST", body: { subject_id: input.subjectId, confirmed: true } },
      ),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "inventory"] });
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

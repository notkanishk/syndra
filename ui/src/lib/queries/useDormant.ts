"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { OneShotSecret } from "@/lib/secret";

/**
 * Accounts Syndra created whose reason for existing has gone (9.11/9.12).
 *
 * Dormancy is a backend judgement, never a frontend filter: the rule is that
 * anything an active role still grants never appears here, and a client
 * computing that from two lists would be re-deriving the resolver. The first
 * time the two disagreed, the disagreement would be somebody losing an account
 * a role still confers.
 */

export type DormantReason =
  | "membership_ended"
  | "role_deleted"
  | "mapping_removed"
  | "never_assigned";

export interface DormantAccount {
  account: string;
  subject_id?: string;
  display_name?: string;
  reason: DormantReason;
  /** What separates housekeeping from a lockout. The surface refuses to act on it. */
  subject_still_member: boolean;
  last_seen_at: string;
  /**
   * Absent when the target does not report per-account usage. "Unknown" and
   * "nothing" are different answers, and only one of them belongs in a sentence
   * an operator ticks.
   */
  bytes_held?: number;
}

export interface DormantReport {
  target: string;
  state_read_at: string;
  truncated: boolean;
  accounts: DormantAccount[];
}

export function useDormantAccounts(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "dormant"],
    queryFn: () => request<DormantReport>(`/targets/${target}/accounts/dormant`),
    enabled: Boolean(target),
  });
}

export interface SweepOutcome {
  account: string;
  outcome: "removed" | "refused" | "unreached" | "indeterminate";
  detail?: string;
}

export interface SweepResult {
  target: string;
  removed: number;
  outcomes: SweepOutcome[];
}

/**
 * Removing them, one dispatch each.
 *
 * The elevated credential is supplied by the operator and travels no further
 * than this request: the add-on holds no delete-capable credential of its own,
 * which is why a compromise of it cannot destroy anybody's files.
 *
 * It arrives in a one-shot box rather than as a plain string, and that is the
 * difference between the sentence above being true and being a claim. A
 * mutation's variables live in the MutationCache after the request settles, so
 * passing the key directly left the one delete-capable credential in this
 * deployment sitting in memory behind a comment saying it was kept nowhere.
 */
export function useSweepDormant(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { accounts: string[]; elevatedKey: OneShotSecret }) =>
      request<SweepResult>(`/targets/${target}/accounts/dormant/sweep`, {
        method: "POST",
        body: { accounts: input.accounts, elevated_key: input.elevatedKey.take(), confirmed: true },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "dormant"] });
      client.invalidateQueries({ queryKey: ["targets", target, "inventory"] });
    },
  });
}

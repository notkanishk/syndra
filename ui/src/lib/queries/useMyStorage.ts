"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { OneShotSecret } from "@/lib/secret";

/**
 * A member's own storage view.
 *
 * Three states, and the page renders them as three. "Entitled with no account
 * yet" is the one a two-state design collapses, and it is the ordinary
 * experience of every new member until an operator resumes the drain.
 */

export interface MyTargetView {
  target: string;
  /** At least one mapped role reaches this target for this person. */
  entitled: boolean;
  /** The add-on-reported name they connect with, or absent. */
  account?: { username: string; bound_at?: string };
  /** What their current entitlements reach — only what resolves. */
  resources?: Record<string, string[]>;
  /** What an operator has withheld, with the reason, so "no access" is never unexplained. */
  suspended?: Array<{ field: string; value: string; reason: string; actor_id: string }>;
  credential: {
    set: boolean;
    /**
     * Enrolled before the LLDAP bridge was retired. Their old credential is
     * gone with the system it was for, so they have to set a new one — and
     * "you enrolled before the change" is a different sentence from "you have
     * never set one" to somebody who remembers doing it.
     */
    needs_re_enrolment?: boolean;
    last_changed_at?: string;
  };
  /** The add-on answered. A member whose target is down is told, not shown a form that fails. */
  reachable: boolean;
  /**
   * The state an operator put the target in: `active`, `draining` or
   * `read_only`.
   *
   * Not a boolean, and the two pauses are not interchangeable. A drain is
   * minutes and ends by itself; read-only is somebody working on the server and
   * ends when they say so. One earns the word "shortly" and the other must
   * never be given it — an estimate attached to an open-ended pause is the
   * small lie that makes the rest of a page untrustworthy.
   */
  lifecycle?: "active" | "draining" | "read_only";
  /**
   * When their access was written down, for the middle state.
   *
   * The wait is on a person resuming the drain, not on a timer, so the page
   * states an age rather than an estimate. "This usually clears within a day"
   * is a guess about somebody's week; "recorded two days ago" is true.
   */
  recorded_at?: string;
  /**
   * How to reach it, from the add-on's own registration.
   *
   * Absent when the deployment has not named a share host, and the page then
   * omits the instructions rather than printing one that does not answer.
   */
  connection?: { protocol: string; host: string };
  /**
   * What the TARGET says about this account right now.
   *
   * Not the same as `credential` above, which is Syndra's record that a
   * password was set. That record cannot say whether the target still accepts
   * it — and an account Syndra created has password authentication disabled
   * until its member sets one, so it exists, is correct, and refuses them.
   */
  storage?: {
    username: string;
    /** Whether they can connect right now. */
    usable: boolean;
    /** The one action that fixes it, when that is what is wrong. */
    needs_password: boolean;
    smb_enabled: boolean;
    shares?: Array<{ share: string; used_bytes: number; quota_bytes?: number }>;
    /** "Nothing used" and "could not look" are the same zero without this. */
    usage_readable?: boolean;
  };
}

export function useMyStorage() {
  return useQuery({
    queryKey: ["me", "targets"],
    queryFn: () => request<{ targets: MyTargetView[] }>("/me/targets"),
    select: (data) => data.targets ?? [],
  });
}

/**
 * Setting the storage credential.
 *
 * The value goes to the target and is kept nowhere: not in the query cache, not
 * in the response, not in Syndra's database. What comes back is metadata.
 *
 * The one-shot box is what makes the first of those true. A mutation's
 * variables outlive the request in the MutationCache, so taking the password as
 * a plain argument retained it — under this exact sentence.
 */
export function useSetStorageCredential(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (password: OneShotSecret) =>
      request<{ status: string; detail: string }>(`/me/targets/${target}/credential`, {
        method: "POST",
        body: { password: password.take() },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["me", "targets"] });
    },
  });
}

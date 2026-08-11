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

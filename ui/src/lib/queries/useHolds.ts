"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import { usersQueryKeys } from "@/lib/queries/useUsers";

/**
 * Holds — access a role still grants, taken away without touching the role.
 *
 * **The word in the interface is "hold".** The API calls this object an
 * allowance, which reads as permission granted when it does the opposite;
 * members already read the state as *withheld*, and *hold* is the same root as
 * a noun. The API name stays here in the code and never reaches a screen.
 *
 * It is one object with three faces: authored on the row it holds, listed under
 * Review once its date passes, and read by the member on their own page.
 */

export interface Hold {
  id: string;
  subject_id: string;
  target: string;
  field: string;
  value: string;
  direction: string;
  actor_id: string;
  reason: string;
  expires_at?: string;
  review_date?: string;
  created_at: string;
  ended?: string;
  ended_by?: string;
}

export interface HoldRow {
  allowance: Hold;
  in_force: boolean;
  review_due: boolean;
}

export function useSubjectHolds(subjectId: string | undefined) {
  return useQuery({
    queryKey: ["users", subjectId, "allowances"],
    queryFn: () => request<{ allowances: HoldRow[] }>(`/users/${subjectId}/allowances`),
    enabled: Boolean(subjectId),
    select: (data) => data.allowances ?? [],
  });
}

/**
 * Holds whose review date has passed.
 *
 * Its own queue beside Expiring access, never a row inside it, because inaction
 * means the opposite thing in each: an expiring grant lapses if ignored, and a
 * hold STAYS IN FORCE. One list would sit "do nothing and access ends" next to
 * "do nothing and access stays blocked".
 */
export function useHoldsDueForReview() {
  return useQuery({
    queryKey: ["governance", "holds", "review-due"],
    queryFn: () =>
      request<{ due_for_review: Hold[]; count: number }>("/governance/allowances/review-due"),
  });
}

export interface NewHold {
  subjectId: string;
  target: string;
  field: string;
  value: string;
  reason: string;
  /** It lifts itself on this date. */
  expiresAt?: string;
  /** It stays until somebody decides, and surfaces for that decision on this date. */
  reviewDate?: string;
}

export function useCreateHold() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: NewHold) =>
      request<Hold>("/allowances", {
        method: "POST",
        body: {
          subject_id: input.subjectId,
          target: input.target,
          field: input.field,
          value: input.value,
          // Always subtractive from this surface. The direction exists in the
          // API for a grant-shaped allowance nothing authors yet, and offering
          // the choice here would offer a way to grant access from a control
          // labelled "hold".
          direction: "deny",
          reason: input.reason,
          expires_at: input.expiresAt,
          review_date: input.reviewDate,
        },
      }),
    onSuccess: (hold) => {
      // Through the key factory, never hand-written. This used to invalidate
      // `["users", subject_id]`, which matches nothing: the access query is
      // keyed `["users", "access", id]` and prefix matching compares position
      // by position, so authoring a hold left the Withheld band off the page
      // that had just authored it. Lifting one used a bare `["users"]` and did
      // refresh — one direction working and the other not is what makes it a
      // bug rather than a convention.
      client.invalidateQueries({ queryKey: usersQueryKeys.access(hold.subject_id) });
      client.invalidateQueries({ queryKey: usersQueryKeys.grants(hold.subject_id) });
      // And the hold list itself, which IS keyed under the subject — the one
      // thing the old key happened to match, which is why this looked like it
      // worked.
      client.invalidateQueries({ queryKey: ["users", hold.subject_id] });
      client.invalidateQueries({ queryKey: ["governance", "holds", "review-due"] });
      client.invalidateQueries({ queryKey: ["me", "targets"] });
    },
  });
}

export function useLiftHold() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      request<{ status: string }>(`/allowances/${id}/lift`, { method: "POST", body: {} }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["users"] });
      client.invalidateQueries({ queryKey: ["governance", "holds", "review-due"] });
      client.invalidateQueries({ queryKey: ["me", "targets"] });
    },
  });
}

export interface RevocationResult {
  status: "revoked" | "partially_revoked";
  allowance_id: string;
  rotated: boolean;
  operation?: string;
  queued?: boolean;
  disclosure?: string;
  detail: string;
  outstanding?: string;
}

/**
 * Taking access away on a target: the hold and the credential rotation, as one
 * action.
 *
 * The backend composes it out of the only two things the target can actually
 * do, and fixes the copy that says what neither of them achieves. A UI that
 * paraphrased that sentence would be paraphrasing a security guarantee.
 */
export function useRevokeTargetAccess(target: string, subjectId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { reason: string; reviewDate?: string }) =>
      request<RevocationResult>(`/targets/${target}/users/${subjectId}/revoke-access`, {
        method: "POST",
        body: { reason: input.reason, confirmed: true, review_date: input.reviewDate },
      }),
    onSuccess: () => {
      // Same three as authoring a hold: a revocation writes one, and the
      // access view is keyed apart from the subject's other queries.
      client.invalidateQueries({ queryKey: usersQueryKeys.access(subjectId) });
      client.invalidateQueries({ queryKey: usersQueryKeys.grants(subjectId) });
      client.invalidateQueries({ queryKey: ["users", subjectId] });
      client.invalidateQueries({ queryKey: ["governance", "unconfirmed-revocations"] });
      client.invalidateQueries({ queryKey: ["governance", "indicators"] });
    },
  });
}

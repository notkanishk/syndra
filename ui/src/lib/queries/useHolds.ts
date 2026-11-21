"use client";

import { settleIndicators } from "@/lib/queries/useIndicators";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import { invalidateAfter } from "./invalidation";

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
    onSuccess: () => invalidateAfter(client, "access"),
  });
}

export function useLiftHold() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      request<{ status: string }>(`/allowances/${id}/lift`, { method: "POST", body: {} }),
    onSuccess: () => {
      settleIndicators(client, { holds_due: -1 });
      invalidateAfter(client, "access");
    },
  });
}

/**
 * A revocation's reply (design §13.4): *recorded* now, *landed* per system only
 * when the add-on answers that it applied it. Between the two the account there
 * still works, and the reply says so in `detail`.
 */
export interface RevocationResult {
  status: "recorded";
  allowance_id: string;
  recorded_at: string;
  /** Always null on the reply: nothing lands before the add-on answers. */
  landed_at: string | null;
  /** The credential half: replaced now, or `outstanding` with how `rotation` went. */
  rotated: boolean;
  operation?: string;
  /** Recorded, not yet applied, why when the system is held down, that the account still works, and that Syndra applies it by itself. */
  detail: string;
  outstanding?: "password.rotate";
  /**
   * The credential half that did not land: refused or never sent (with its own sentence), or
   * a write's two answers (a NOTHING_SENT envelope, or `{status: "unconfirmed"}`).
   */
  rotation?: { status?: "not_sent" | "refused" | "unconfirmed"; detail?: string; error?: string; message?: string };
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
    onSuccess: () => invalidateAfter(client, "access"),
  });
}

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

export interface AccessRequest {
  id: string;
  requester_id: string;
  project_id: string;
  role_key: string;
  justification: string;
  duration_days?: number | null;
  status: "pending" | "approved" | "rejected" | "withdrawn" | "fulfilled" | string;
  reviewer_id?: string;
  review_note?: string;
  created_at: string;
  /** When it was decided, withdrawn, or closed by Syndra because it was given another way. */
  resolved_at?: string | null;
  /** "extend" asks to keep a role they hold longer; absent means an ordinary ask. */
  kind?: "access" | "extend";
  /** What the requester already has of the role, read for a waiting request. */
  holding?: RequestHolding;
  /** A waiting extension only: the end they have now, and the end approving would set. Absent when unknown. */
  current_until?: string;
  until?: string;
}

/** What someone already has of a role they ask for. */
export interface RequestHolding {
  kind: "none" | "until" | "for_good";
  /** When it ends, for "until". */
  until?: string;
  /** Given, and not yet sent to Zitadel. */
  waiting?: boolean;
}

export interface CreateRequestInput {
  /** Admin-only: omit for self-service member submissions. */
  requester_id?: string;
  project_id: string;
  role_key: string;
  justification: string;
  duration_days: number;
  /** Derived from what the member holds: "extend" for a role with an end date. */
  kind?: "access" | "extend";
}

export interface DecisionInput {
  status: "approved" | "rejected";
  review_note?: string;
}

/** What deciding returns: an approval that isn't in Zitadel yet says why. */
export interface DecisionResult {
  message?: string;
  wait_reason?: string;
  /** given, the end moved (extended), or it had ended while waiting (given_again). */
  effect?: "given" | "extended" | "given_again";
  until?: string;
}

const KEYS = {
  byStatus: (status: string) => ["requests", { status }] as const,
  all: ["requests"] as const,
};

/** Admin queue. `status: "all"` collapses the filter. */
export function useRequestsAdmin(status: string = "pending") {
  return useQuery({
    queryKey: KEYS.byStatus(status),
    queryFn: async (): Promise<AccessRequest[]> => {
      const path = status === "all" ? "/requests" : `/requests?status=${encodeURIComponent(status)}`;
      const data = await request<unknown>(path);
      return Array.isArray(data) ? (data as AccessRequest[]) : [];
    },
  });
}

/**
 * The caller's own requests. A member's list is always theirs; `?mine=1` asks
 * the same of an operator, whose plain read is the whole queue. Kept under its
 * own key so the unfiltered queue's cache stays the queue's.
 */
export function useRequestsMine() {
  return useQuery({
    queryKey: KEYS.byStatus("mine"),
    queryFn: async (): Promise<AccessRequest[]> => {
      const data = await request<unknown>("/requests?mine=1");
      return Array.isArray(data) ? (data as AccessRequest[]) : [];
    },
  });
}

/** Create a request. Members omit `requester_id`; the proxy stamps it. */
export function useCreateRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateRequestInput) => {
      return await request<AccessRequest>("/requests", { method: "POST", body: input });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["requests"] });
    },
  });
}

/**
 * Approve / reject a pending request. The backend resolves the reviewer from
 * the authenticated principal (or the proxy injects the demo session id);
 * callers do not pass `reviewer_id`.
 */
export function useDecideRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, ...input }: DecisionInput & { id: string }) => {
      return await request<DecisionResult>(`/requests/${id}/decision`, {
        method: "POST",
        body: input,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["requests"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

/**
 * Take back your own pending request.
 *
 * Not a rejection: nobody refused it, and a refusal in the log for something the person changed
 * their mind about is a record of an event that did not happen. The backend binds this to the
 * requester on the row, so there is no id to pass beyond the request's own.
 *
 * `governance` is invalidated alongside `requests` because the withdrawal leaves the operator's
 * queue — the sidebar badge is counting it until it hears otherwise.
 */
export function useWithdrawRequest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) =>
      request<{ message: string }>(`/requests/${id}/withdraw`, { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["requests"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}

export const requestsQueryKeys = KEYS;

/**
 * Bulk approve / deny, rehearsed.
 *
 * Returns the same plan shape every other bulk surface returns, so the queue
 * shares one renderer and one vocabulary with People and the drift triage.
 * Approving in bulk has the least obvious blast radius on the product: each
 * approval mints a direct grant, so "approve 9" is nine access changes wearing
 * the clothes of an inbox action.
 */
export interface BulkDecisionInput {
  ids: string[];
  status: "approved" | "rejected";
  review_note?: string;
  /**
   * The operator confirming an affected-subject count above the configured
   * limit. Sent on the rehearsal, never on the apply: it unlocks issuing the
   * approval rather than changing what the approval does.
   */
  acknowledge_scope?: boolean;
  /** Cites the rehearsal being applied. Set by the apply pass, never composed by hand. */
  plan_id?: string;
}

function useBulkDecisionMutation(apply: boolean) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BulkDecisionInput) =>
      request<BulkPlan>(apply ? "/requests/bulk-decision?apply=true" : "/requests/bulk-decision", {
        method: "POST",
        body: input,
      }),
    onSuccess: () => {
      if (!apply) return;
      qc.invalidateQueries({ queryKey: ["requests"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export const useRehearseBulkDecision = () => useBulkDecisionMutation(false);
export const useApplyBulkDecision = () => useBulkDecisionMutation(true);

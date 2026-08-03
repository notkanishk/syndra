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
  status: "pending" | "approved" | "rejected" | "withdrawn" | string;
  reviewer_id?: string;
  review_note?: string;
  created_at: string;
}

export interface CreateRequestInput {
  /** Admin-only: omit for self-service member submissions. */
  requester_id?: string;
  project_id: string;
  role_key: string;
  justification: string;
  duration_days: number;
}

export interface DecisionInput {
  status: "approved" | "rejected";
  review_note?: string;
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
 * Member view — lists only the caller's own requests. The proxy filters the
 * response server-side so we don't need to scope client-side, and the
 * unfiltered `requests` key stays shared with the admin queue cache.
 */
export function useRequestsMine() {
  return useQuery({
    queryKey: KEYS.byStatus("mine"),
    queryFn: async (): Promise<AccessRequest[]> => {
      const data = await request<unknown>("/requests");
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
      return await request<AccessRequest>(`/requests/${id}/decision`, {
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

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface MappingRuleRow {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  confirmation_mode?: "auto" | "manual";
  /** How many people hold the input role — i.e. how many this rule acts on. */
  holder_count: number;
  created_at: string;
}

export interface CreateMappingRuleInput {
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  /** Optional override — omitted falls back to the global default (Task 22). */
  confirmation_mode?: "auto" | "manual";
}

export interface ValidateMappingRuleResult {
  would_cycle: boolean;
  self_reference: boolean;
  reason?: string;
}

const KEYS = {
  list: ["mapping-rules"] as const,
};

/** List all active mapping rules. Backs /policies and /projects. */
export function useMappingRules() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: async (): Promise<MappingRuleRow[]> => {
      const data = await request<unknown>("/rules/mapping");
      return Array.isArray(data) ? (data as MappingRuleRow[]) : [];
    },
  });
}

/** Create a new mapping rule. Invalidates the list. */
export function useCreateMappingRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateMappingRuleInput) => {
      return await request<MappingRuleRow>("/rules/mapping", {
        method: "POST",
        body: input,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}

/**
 * Retarget an existing rule. The backend recomputes the closure diff and
 * enqueues the adds and revokes in one transaction, so a retarget is never
 * half-applied — which is exactly the failure that produces access nobody can
 * explain later.
 */
export function useUpdateMappingRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, ...input }: CreateMappingRuleInput & { id: string }) =>
      request<MappingRuleRow>(`/rules/mapping/${id}`, { method: "PUT", body: input }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

/**
 * Change when a rule's writes reach the identity provider. Uses the bulk
 * endpoint with a single id — there is no per-rule route, and inventing one
 * for a set of size one would be a second way to do the same thing.
 */
export function useSetRuleConfirmationMode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, mode }: { id: string; mode: "auto" | "manual" }) =>
      request("/policies/confirmation-mode", {
        method: "POST",
        body: { kind: "rule", ids: [id], mode },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}

/**
 * Validate a draft rule. Returns cycle / self-reference flags so the create
 * form can warn before submission. Stateless on the backend — no cache.
 */
export function useValidateMappingRule() {
  return useMutation({
    mutationFn: async (input: CreateMappingRuleInput) => {
      return await request<ValidateMappingRuleResult>("/rules/mapping/validate", {
        method: "POST",
        body: input,
      });
    },
  });
}

export const mappingRulesQueryKeys = KEYS;

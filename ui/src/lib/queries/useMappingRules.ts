"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface MappingRuleRow {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  created_at: string;
}

export interface CreateMappingRuleInput {
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
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

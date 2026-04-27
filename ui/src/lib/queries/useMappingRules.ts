"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface MappingRuleRow {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  version: number;
  created_at: string;
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

export const mappingRulesQueryKeys = KEYS;

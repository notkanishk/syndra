"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

import { bundlesQueryKeys } from "./useBundles";
import { mappingRulesQueryKeys } from "./useMappingRules";

export type ConfirmationMode = "auto" | "manual";

export interface CascadeSummaryRow {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  source: string;
  source_ref?: string;
  cascade_id?: string;
  status: string;
  completed_at?: string;
}

/**
 * Change history's unit: every write ONE triggering event produced, collapsed
 * into a single entry. "8 applied", "2 waiting", "no writes" is the whole
 * vocabulary — an entry is a sentence about consequence, not a diff.
 */
export interface CascadeGroupRow {
  cascade_id: string;
  source: string;
  source_ref?: string;
  applied: number;
  waiting: number;
  failed: number;
  user_ids: string[];
  writes: CascadeSummaryRow[];
  started_at: string;
  settled_at?: string | null;
}

const KEYS = {
  globalDefault: ["config", "confirmation-mode-default"] as const,
  cascadeGroups: ["propagations", "cascade-groups"] as const,
};

/** The operator-configured global default confirmation mode for new rules/bundles. */
export function useGlobalConfirmationDefault() {
  return useQuery({
    queryKey: KEYS.globalDefault,
    queryFn: async () =>
      (await request<{ mode: ConfirmationMode }>("/config/confirmation-mode-default")).mode,
  });
}

/** Operator mutation: changes the global default confirmation mode. */
export function useSetGlobalConfirmationDefault() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (mode: ConfirmationMode) =>
      request<{ mode: ConfirmationMode }>("/config/confirmation-mode-default", {
        method: "PUT",
        body: { mode },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.globalDefault });
    },
  });
}

export interface BulkSetConfirmationModeInput {
  kind: "rule" | "bundle";
  ids: string[];
  mode: ConfirmationMode;
}

/**
 * Bulk-toggle confirmation_mode on a set of rules or bundles in one call.
 * Invalidates both list caches — the caller passes `kind`, so only one of the
 * two actually changed, but invalidating both is cheap and keeps this hook
 * from having to know which list is currently mounted.
 */
export function useBulkSetConfirmationMode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: BulkSetConfirmationModeInput) =>
      request("/policies/confirmation-mode", { method: "POST", body: input }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: mappingRulesQueryKeys.list });
      qc.invalidateQueries({ queryKey: bundlesQueryKeys.list });
    },
  });
}

/**
 * Change history. Includes cascades whose writes have NOT landed: a half-applied cascade is the thing that creates
 * unexplained access, and it has to be visible as one.
 */
export function useCascadeGroups(cascadeId?: string) {
  const scope = cascadeId ? `?cascade=${encodeURIComponent(cascadeId)}` : "";
  return useQuery({
    queryKey: [...KEYS.cascadeGroups, cascadeId ?? ""] as const,
    queryFn: async () =>
      (await request<{ cascades: CascadeGroupRow[] }>(`/propagations/cascade-groups${scope}`))
        .cascades ?? [],
  });
}

export const confirmationModeQueryKeys = KEYS;

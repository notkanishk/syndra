"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

import { governanceQueryKeys, type DriftItem } from "./useGovernance";

export interface DriftFilter {
  user_id?: string;
  project_id?: string;
  source?: string;
}

const KEYS = {
  list: (f?: DriftFilter) => ["drift", "list", f ?? {}] as const,
};

/** The operator's drift worklist — out-of-band changes needing triage. */
export function useDriftItems(filter?: DriftFilter) {
  const qs = new URLSearchParams(
    Object.entries(filter ?? {}).filter(([, v]) => v) as [string, string][],
  ).toString();
  return useQuery({
    queryKey: KEYS.list(filter),
    queryFn: async () =>
      (await request<{ drift: DriftItem[] }>(`/governance/drift${qs ? `?${qs}` : ""}`)).drift ?? [],
  });
}

/**
 * Shared shape for the drift triage actions (attribute/revoke/mark-external):
 * all POST to a per-item endpoint and invalidate both the drift list and the
 * governance summary so the nav badge and dashboard callout update immediately.
 */
function useDriftMutation<B>(path: (id: string) => string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body?: B }) =>
      request(path(id), { method: "POST", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["drift"] });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

export const useAttributeDrift = () =>
  useDriftMutation<{ source: string; source_ref?: string }>(
    (id) => `/governance/drift/${id}/attribute`,
  );
export const useRevokeDrift = () =>
  useDriftMutation<undefined>((id) => `/governance/drift/${id}/revoke`);
export const useMarkExternalDrift = () =>
  useDriftMutation<{ reason?: string }>((id) => `/governance/drift/${id}/mark-external`);

/** The operator's "Reconcile now" action: forces an immediate drift scan. */
export function useReconcileNow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => request("/governance/drift/reconcile", { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["drift"] });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

export const driftQueryKeys = KEYS;

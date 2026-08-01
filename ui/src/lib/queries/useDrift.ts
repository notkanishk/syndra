"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

import { governanceQueryKeys, type DriftItem } from "./useGovernance";

export interface DriftFilter {
  user_id?: string;
  project_id?: string;
  source?: string;
}

/**
 * A drift row with everything the triage queue needs on the row itself. The
 * backend orders these by risk then age — a safety-gated role found yesterday
 * outranks a wiki role found last week — so the UI must NOT re-sort them.
 */
export interface DriftTriageItem extends DriftItem {
  zitadel_grant_id?: string;
  upstream_actor?: string;
  upstream_created_at?: string | null;
  last_seen_at?: string | null;
  role_group?: string;
  role_in_catalogue: boolean;
  user_status?: string;
  user_is_service_account: boolean;
  other_items_for_user: number;
}

/**
 * The only attribution the backend can honour.
 *
 * "bundle" and "rule" were once accepted and are not any more: adopting writes a
 * direct grant, and cannot assign a bundle to somebody or create a rule-derived
 * relationship. A grant labelled as bundle-owned but not actually owned by one
 * survives that bundle's removal, and the ledger claims otherwise.
 */
export type AttributionSource = "external_backfill";

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
      (await request<{ drift: DriftTriageItem[] }>(`/governance/drift${qs ? `?${qs}` : ""}`))
        .drift ?? [],
  });
}

/**
 * Bulk adopt / bulk mark-as-external, rehearsed.
 *
 * Both return the same `BulkPlan` every other bulk surface returns, so the
 * triage queue and the People page share one renderer and one vocabulary
 * rather than each explaining "what will change" in its own words.
 *
 * There is still deliberately no bulk revoke: adopting and marking-external are
 * reversible bookkeeping, but revoking removes real access from real machines,
 * and reading twelve consequences at once is not something anyone actually
 * does. Revoke stays one row, one dialog, one decision.
 */
function useBulkDriftMutation<B>(path: string, apply: boolean) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: B) =>
      request<BulkPlan>(apply ? `${path}?apply=true` : path, { method: "POST", body }),
    onSuccess: () => {
      if (!apply) return;
      qc.invalidateQueries({ queryKey: ["drift"] });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

type AdoptBody = { ids: string[]; source: AttributionSource };
type ExternalBody = { ids: string[]; reason: string };

export const useRehearseAdoptDrift = () =>
  useBulkDriftMutation<AdoptBody>("/governance/drift/bulk-attribute", false);
export const useBulkAttributeDrift = () =>
  useBulkDriftMutation<AdoptBody>("/governance/drift/bulk-attribute", true);

export const useRehearseMarkExternalDrift = () =>
  useBulkDriftMutation<ExternalBody>("/governance/drift/bulk-mark-external", false);
export const useBulkMarkExternalDrift = () =>
  useBulkDriftMutation<ExternalBody>("/governance/drift/bulk-mark-external", true);

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
  useDriftMutation<{ source: AttributionSource }>(
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

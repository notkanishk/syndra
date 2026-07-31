"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

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
 * What a bulk resolution actually did.
 *
 * `failed_ids` names the rows that did NOT resolve, so the caller can keep
 * exactly those selected. A count alone leaves only bad options: re-send
 * everything and redo work that succeeded, or quietly drop the failures — and
 * the second one is how access nobody resolved gets reported as handled.
 */
export interface BulkDriftResult {
  /** Present on bulk-attribute. */
  attributed?: number;
  /** Present on bulk-mark-external. */
  marked?: number;
  failed: number;
  failed_ids: string[];
}

function useBulkDriftMutation<B>(path: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: B) => {
      const raw = await request<Partial<BulkDriftResult>>(path, { method: "POST", body });
      return {
        attributed: raw?.attributed,
        marked: raw?.marked,
        failed: raw?.failed ?? 0,
        failed_ids: Array.isArray(raw?.failed_ids) ? raw.failed_ids : [],
      } satisfies BulkDriftResult;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["drift"] });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

/**
 * Bulk adopt / bulk mark-as-external. There is deliberately no bulk revoke:
 * adopting is reversible bookkeeping, but revoking removes real access from
 * real machines, and reading twelve consequences at once is not something
 * anyone actually does. Revoke stays one row, one dialog, one decision.
 */
export const useBulkAttributeDrift = () =>
  useBulkDriftMutation<{ ids: string[]; source: AttributionSource }>(
    "/governance/drift/bulk-attribute",
  );

export const useBulkMarkExternalDrift = () =>
  useBulkDriftMutation<{ ids: string[]; reason: string }>(
    "/governance/drift/bulk-mark-external",
  );

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

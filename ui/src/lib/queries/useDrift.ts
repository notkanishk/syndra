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
  /**
   * False on a target with no role catalogue at all. `role_in_catalogue` is
   * then meaningless rather than false — nothing was retired, because there was
   * never a catalogue to retire it from. Always read the two together.
   */
  role_catalogue_applies: boolean;
  user_status?: string;
  user_is_service_account: boolean;
  other_items_for_user: number;
  /** Where the access came from, for a row about access Syndra intends. It is
   * what makes a removal legible: the same entitlement Syndra applied, not a
   * finding that appeared from nowhere. */
  provenance?: GrantProvenance;
}

/** The decision behind an entitlement, and when the target was last seen
 * holding it. */
export interface GrantProvenance {
  granted_by?: string;
  granted_at?: string;
  reason?: string;
  source?: string;
  source_ref?: string;
  last_observed_at?: string;
  expires_at?: string;
  /** When the target ACCEPTED Syndra's write, and who it was attributed to.
   * The only evidence that exists for a grant applied and removed between two
   * sweeps — no read ever saw that one. */
  applied_at?: string;
  applied_by?: string;
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

/**
 * `plan_id` cites the rehearsal being applied. Optional on the type because one
 * body serves both passes; the apply pass always sets it, and the backend
 * refuses an apply without one.
 */
type AdoptBody = {
  ids: string[];
  source: AttributionSource;
  plan_id?: string;
  acknowledge_scope?: boolean;
};
type ExternalBody = {
  ids: string[];
  reason: string;
  plan_id?: string;
  acknowledge_scope?: boolean;
};

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

/**
 * Who created a grant, read from Zitadel's own event log.
 *
 * The sweep compares grant SETS, so a row it raised carries no `upstream_actor`
 * and says "unknown". That is honest and it is the least useful sentence on the
 * queue. Zitadel is event-sourced and can answer exactly — which is why the
 * Zitadel side gets this rather than the recorded merge base the add-on targets
 * use. A base infers who moved from a snapshot difference; this is written down.
 *
 * `enabled` is off until somebody asks. Drift arrives in clusters and the queue
 * routinely holds dozens of rows; firing one API call per row on load would
 * turn opening a page into a burst against the identity provider.
 */
export interface DriftOrigin {
  id: string;
  /** False when Zitadel could not be asked at all. Never confuse with `recorded`. */
  readable: boolean;
  /** False when Zitadel answered and holds no event this far back. */
  recorded?: boolean;
  /** False when the event exists and names nobody. A real answer, not a gap. */
  attributed?: boolean;
  actor_id?: string;
  actor_name?: string;
  /** The machine actor when there is no human one — an Action, a service account. */
  service?: string;
  event_type?: string;
  at?: string;
  detail?: string;
}

export function useDriftOrigin(id: string, enabled: boolean) {
  return useQuery({
    queryKey: ["drift", id, "origin"],
    queryFn: () => request<DriftOrigin>(`/governance/drift/${encodeURIComponent(id)}/origin`),
    enabled: enabled && Boolean(id),
    // An event that already happened does not change. Re-asking costs the
    // identity provider a round trip and can never return anything new.
    staleTime: Infinity,
  });
}

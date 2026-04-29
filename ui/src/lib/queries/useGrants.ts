"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Single Zitadel-side grant. Mirrors `zitadel.UserGrant`. JSON keys are
 * camelCase because the backend forwards Zitadel's payload verbatim — keep
 * the type shape aligned with the wire format.
 */
export interface ZitadelGrantRow {
  id: string;
  userId: string;
  projectId: string;
  roleKeys: string[];
}

interface ZitadelGrantsPageEnvelope {
  items: ZitadelGrantRow[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * Aggregated result of paging through every Zitadel grant. `truncated` is
 * true only when the safety cap halted iteration before the inventory was
 * exhausted; in that case the All-grants ledger is incomplete and the UI
 * MUST surface a warning.
 */
export interface ZitadelGrantsAggregate {
  items: ZitadelGrantRow[];
  total: number;
  truncated: boolean;
}

/** Per-page limit issued to the proxy. Below the documented Zitadel cap. */
const ZITADEL_PAGE_SIZE = 500;

/**
 * Bounds the client-side fetch so a pathologically large directory cannot
 * stall the page or balloon memory. Past this point, the hook stops paging
 * and surfaces `truncated: true` so the All-grants tab can warn the operator
 * the ledger is partial.
 */
const ZITADEL_AGGREGATE_CAP = 5_000;

/**
 * Reconciliation diff shape mirrors `handlers.ReconciliationDiff`.
 *  - only_in_mkauth — direct grants with no Zitadel counterpart
 *  - only_in_zitadel — Zitadel grants not tracked by MkAuth (mapping-rule
 *    derivatives or pre-existing manual grants)
 *  - drift — same (user, project) on both sides but role sets disagree
 */
export interface ReconciliationGrant {
  user_id: string;
  project_id: string;
  role_keys: string[];
  grant_id?: string;
}

export interface ReconciliationDriftEntry {
  user_id: string;
  project_id: string;
  mkauth_roles: string[];
  zitadel_roles: string[];
  only_in_mkauth: string[];
  only_in_zitadel: string[];
  grant_id?: string;
}

export interface ReconciliationDiff {
  only_in_mkauth: ReconciliationGrant[];
  only_in_zitadel: ReconciliationGrant[];
  drift: ReconciliationDriftEntry[];
  generated_at: string;
  truncated: boolean;
}

const KEYS = {
  zitadelAll: ["grants", "zitadel", "aggregate"] as const,
  reconciliation: ["grants", "reconciliation"] as const,
};

/**
 * Aggregates every Zitadel-side user grant by paging the proxy until the
 * inventory is exhausted or the safety cap halts iteration. The previous
 * single-page implementation silently dropped grants past offset=500 — the
 * All-grants ledger MUST reflect every active grant or operators miss live
 * assignments. `truncated` flags the operator-facing partial-snapshot copy.
 */
export function useZitadelAllGrants() {
  return useQuery({
    queryKey: KEYS.zitadelAll,
    queryFn: async (): Promise<ZitadelGrantsAggregate> => {
      const all: ZitadelGrantRow[] = [];
      let offset = 0;
      let total = 0;
      let truncated = false;
      // Loop bounded by the safety cap; defensive break on an empty page in
      // case the proxy regression returns a stable empty payload past the end.
      while (true) {
        const raw = await request<unknown>(
          `/zitadel/grants?limit=${ZITADEL_PAGE_SIZE}&offset=${offset}`,
        );
        const envelope = raw as Partial<ZitadelGrantsPageEnvelope> | null;
        const items: ZitadelGrantRow[] = Array.isArray(envelope?.items)
          ? envelope.items
          : [];
        total = typeof envelope?.total === "number" ? envelope.total : all.length + items.length;
        all.push(...items);
        if (items.length === 0 || all.length >= total) break;
        if (all.length >= ZITADEL_AGGREGATE_CAP) {
          truncated = true;
          break;
        }
        offset += items.length;
      }
      return { items: all, total, truncated };
    },
  });
}

/**
 * Fetches the reconciliation snapshot. Refetched lazily — operators
 * generally compare a snapshot at a point in time, so this isn't on a polling
 * cadence. The page exposes a "Refresh" affordance that triggers refetch().
 */
export function useReconciliationDiff() {
  return useQuery({
    queryKey: KEYS.reconciliation,
    queryFn: async (): Promise<ReconciliationDiff> => {
      const data = await request<ReconciliationDiff>("/reconciliation/grants");
      return {
        only_in_mkauth: Array.isArray(data?.only_in_mkauth) ? data.only_in_mkauth : [],
        only_in_zitadel: Array.isArray(data?.only_in_zitadel) ? data.only_in_zitadel : [],
        drift: Array.isArray(data?.drift) ? data.drift : [],
        generated_at: data?.generated_at ?? "",
        truncated: !!data?.truncated,
      };
    },
  });
}

export const grantsQueryKeys = KEYS;

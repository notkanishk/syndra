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

/**
 * Reconciliation diff shape mirrors `handlers.ReconciliationDiff`.
 *  - only_in_syndra — direct grants with no Zitadel counterpart
 *  - only_in_zitadel — Zitadel grants not tracked by Syndra (mapping-rule
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
  syndra_roles: string[];
  zitadel_roles: string[];
  only_in_syndra: string[];
  only_in_zitadel: string[];
  grant_id?: string;
}

export interface ReconciliationDiff {
  only_in_syndra: ReconciliationGrant[];
  only_in_zitadel: ReconciliationGrant[];
  drift: ReconciliationDriftEntry[];
  generated_at: string;
  truncated: boolean;
}

const KEYS = {
  reconciliation: ["grants", "reconciliation"] as const,
};

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
        only_in_syndra: Array.isArray(data?.only_in_syndra) ? data.only_in_syndra : [],
        only_in_zitadel: Array.isArray(data?.only_in_zitadel) ? data.only_in_zitadel : [],
        drift: Array.isArray(data?.drift) ? data.drift : [],
        generated_at: data?.generated_at ?? "",
        truncated: !!data?.truncated,
      };
    },
  });
}

export const grantsQueryKeys = KEYS;

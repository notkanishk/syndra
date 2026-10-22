"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";

import { request } from "@/lib/api-client";
import type { ServiceAccount, ServiceAccountKind } from "@/lib/types";

/**
 * Service accounts (A-07 §26–31): a Zitadel machine user, or a human-type
 * account an operator marked. The mark lives in Syndra's own database; nothing
 * here writes to Zitadel.
 */

/** The four kinds, in the order the Mark dialog offers them (A-07 §28). */
export const SERVICE_ACCOUNT_KINDS: { value: ServiceAccountKind; label: string; description: string }[] = [
  { value: "integration", label: "Integration", description: "Another system reading or writing through it" },
  { value: "device", label: "Device", description: "A kiosk, panel or station people use in place" },
  { value: "agent", label: "Agent", description: "Software acting on someone's behalf" },
  { value: "shared_login", label: "Shared login", description: "One login several people use" },
];

export const kindLabel = (kind: ServiceAccountKind) =>
  SERVICE_ACCOUNT_KINDS.find((k) => k.value === kind)?.label ?? "Integration";

/** The Account cell's second line: how it gets into Zitadel. */
export const howItSignsIn = (account: Pick<ServiceAccount, "zitadel_type">) =>
  account.zitadel_type === "machine" ? "machine key" : "signs in";

export const serviceAccountHref = (id: string) => `/system/service-accounts/${encodeURIComponent(id)}`;

const KEYS = {
  list: ["service-accounts", "list"] as const,
  one: (id: string) => ["service-accounts", "one", id] as const,
};

export function useServiceAccounts() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: () => request<{ accounts: ServiceAccount[] }>("/service-accounts"),
    select: (data) => data.accounts ?? [],
  });
}

/**
 * Which ids are service accounts, for the rows where access is checked (Drift,
 * Expiring, Holds, a system's accounts). Those reads carry no flag of their own,
 * and one cached list answers for all of them. Until it answers, nobody is one.
 */
export function useServiceAccountIds(): ReadonlySet<string> {
  const { data } = useServiceAccounts();
  return useMemo(() => new Set((data ?? []).map((a) => a.id)), [data]);
}

export function useServiceAccount(id: string) {
  return useQuery({
    queryKey: KEYS.one(id),
    queryFn: () => request<ServiceAccount>(`/service-accounts/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}

export interface ServiceAccountInput {
  kind: ServiceAccountKind;
  purpose: string;
  owner_id: string;
}

/** What a mark or an unmark moves: the account's own reads, and everything that counts people. */
function useRefresh() {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: ["service-accounts"] });
    // People, the people counts and the welcome bundle all read the user list.
    qc.invalidateQueries({ queryKey: ["users"] });
    qc.invalidateQueries({ queryKey: ["governance"] });
  };
}

/** PUT: marks a human-type account, or changes kind, purpose or owner of any service account. */
export function useSaveServiceAccount(id: string) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (input: ServiceAccountInput) =>
      request<ServiceAccount>(`/service-accounts/${encodeURIComponent(id)}`, { method: "PUT", body: input }),
    onSuccess: refresh,
  });
}

/** DELETE: "Mark as a person". The backend refuses a machine account (409 MACHINE_ACCOUNT). */
export function useUnmarkServiceAccount(id: string) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: () => request<unknown>(`/service-accounts/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: refresh,
  });
}

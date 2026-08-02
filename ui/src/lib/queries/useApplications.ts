"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface ApplicationView {
  application: {
    id: string;
    name: string;
    project_id: string;
    description: string;
    consumer: string;
    claim_name: string;
    format_type: string;
  };
  consumed_roles: string[];
  assigned_user_count: number;
}

/**
 * The dry run of a real token. `custom_claims` is the exact map the Actions v2
 * path would append for this (user, project) pair — same profile resolution,
 * same shaper, nothing invented for display.
 *
 * A project's token carries every claim key configured on that project, since
 * Zitadel's function trigger does not say which application the token is for.
 * `owned_claims` narrows that to the keys THIS app reads; `claim_owners`
 * attributes the rest so a sibling's key is never mistaken for a bug.
 */
export interface SimulationResponse {
  application: ApplicationView["application"];
  user: { id: string; name: string; title: string };
  raw_roles: string[];
  custom_claims: Record<string, unknown>;
  owned_claims: string[];
  claim_owners: Array<{
    key: string;
    owner_label: string;
    application_id?: string;
    kind: "roles" | "attribute" | "static";
    source?: string;
  }>;
}

const KEYS = {
  list: ["applications"] as const,
  one: (id: string) => ["applications", id] as const,
  simulate: (appId: string, userId: string) =>
    ["applications", appId, "simulate", userId] as const,
};

/** List all applications. */
export function useApplications() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: async (): Promise<ApplicationView[]> => {
      const data = await request<unknown>("/applications");
      return Array.isArray(data) ? (data as ApplicationView[]) : [];
    },
  });
}

/** Token simulation for (application, user). Cached per pair. */
export function useTokenSimulator(appId: string, userId: string) {
  return useQuery({
    queryKey: KEYS.simulate(appId, userId),
    queryFn: async (): Promise<SimulationResponse | null> => {
      if (!appId || !userId) return null;
      return await request<SimulationResponse>(
        `/applications/${appId}/simulate?user_id=${encodeURIComponent(userId)}`,
      );
    },
    enabled: Boolean(appId && userId),
  });
}

export const applicationsQueryKeys = KEYS;

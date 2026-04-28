"use client";

import { useMutation, useQuery } from "@tanstack/react-query";

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

export interface SimulationResponse {
  application: ApplicationView["application"];
  user: { id: string; name: string; title: string };
  raw_roles: string[];
  custom_claims: Record<string, unknown>;
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

/**
 * Imperative simulation runner — used when callers want a fresh fetch (e.g.
 * the compare-with panel that needs simulations for two distinct users without
 * disturbing the primary cache key). Prefer `useTokenSimulator` for declarative
 * use cases.
 */
export function useSimulateMutation() {
  return useMutation({
    mutationFn: async ({ appId, userId }: { appId: string; userId: string }) => {
      return await request<SimulationResponse>(
        `/applications/${appId}/simulate?user_id=${encodeURIComponent(userId)}`,
      );
    },
  });
}

export const applicationsQueryKeys = KEYS;

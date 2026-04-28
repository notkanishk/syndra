"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface ZitadelHealthResponse {
  status: "ok" | "disabled" | "error" | string;
  mode: "live" | "local-policy-only" | string;
  domain?: string;
  projects_total?: number;
  latency_ms?: number;
  error?: string;
}

const KEYS = {
  health: ["zitadel", "health"] as const,
};

/**
 * Polls the Zitadel diagnostic health endpoint every 10s — the cadence that
 * the operations spec calls out for live infrastructure tiles. The query is
 * paused when the tab is hidden so the background polling cost stays bounded.
 */
export function useZitadelHealth() {
  return useQuery({
    queryKey: KEYS.health,
    queryFn: async (): Promise<ZitadelHealthResponse> => {
      return await request<ZitadelHealthResponse>("/zitadel/health");
    },
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
  });
}

export const zitadelQueryKeys = KEYS;

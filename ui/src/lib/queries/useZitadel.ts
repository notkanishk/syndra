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
      // The backend answers "disabled" (503) and "error" (502) with the same
      // structured payload as "ok" (200) — the error envelope IS the
      // diagnostic. Without this, both non-2xx cases threw ApiError instead,
      // `data` stayed undefined, and the page fell back to one generic
      // "Syndra's server had a problem and gave no reason" for every failure
      // — silently skipping the disabled/unreachable copy below, which never
      // ran against real data.
      return await request<ZitadelHealthResponse>("/zitadel/health", { preserveErrorBody: true });
    },
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
  });
}

export const zitadelQueryKeys = KEYS;

"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * What the TARGET says about itself.
 *
 * Not `useTargetHealth`, which is what the ADD-ON says about the target — is it
 * answering, is the release tested, has its mutation log been rewritten. That
 * one is cheap and polls every 30s. This one costs the add-on four calls to the
 * NAS, so it is read when somebody opens the page and left alone after that.
 *
 * `readable` is load-bearing, and `degraded` is load-bearing inside it. An
 * empty alert list from a target that answered and an empty one from a read
 * that failed are opposite facts, and so are "no alerts" and "the alert source
 * specifically could not be read while the other three could".
 */
export interface TargetSystemHealth {
  target: string;
  readable: boolean;
  system?: { hostname?: string; version?: string; uptime_seconds?: number };
  alerts?: TargetAlert[];
  pools?: PoolStatus[];
  services?: ServiceState[];
  /** Names the sources that could not be read: `system`, `alerts`, `pools`, `services`. */
  degraded?: string[];
  detail?: string;
}

export interface TargetAlert {
  level: string;
  klass?: string;
  /** The target's own prose, with its markup already stripped by the add-on. */
  text: string;
  at?: string;
  dismissed: boolean;
}

export interface PoolStatus {
  name: string;
  status: string;
  healthy: boolean;
  warning: boolean;
  free_bytes: number;
  allocated_bytes: number;
  size_bytes: number;
}

export interface ServiceState {
  service: string;
  state: string;
  enable: boolean;
}

export function useTargetSystemHealth(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "system-health"],
    queryFn: () => request<TargetSystemHealth>(`/targets/${target}/system-health`),
    enabled: Boolean(target),
    // Four calls to the NAS per read. Long enough that moving between tabs on
    // one target does not re-ask, short enough that an operator who fixed
    // something and came back sees it.
    staleTime: 120_000,
  });
}

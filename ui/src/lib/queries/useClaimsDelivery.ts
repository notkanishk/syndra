"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** Home's one line: the same answer the external alert acts on. */
export interface ClaimsStatus {
  status: "ok" | "degraded" | "failing";
  /** Exactly `reasons.length`. */
  problems: number;
  reasons: string[];
  /** Sign-ins before this (a restart) are unchecked, never failed. */
  checking_since: string;
  sign_ins_24h: Partial<Record<"delivered" | "not_called" | "not_served" | "first_sign_in_before_access" | "nothing_to_deliver" | "unchecked", number>>;
  waiting: number;
}

export interface ClaimFinding {
  kind: string;
  severity: "hard" | "soft" | "info";
  subject: string;
  first_seen: string;
  last_seen: string;
  count: number;
  sample?: Record<string, unknown>;
  resolved_at: string | null;
}

export interface ClaimApp {
  client_id: string;
  app_id?: string;
  name?: string;
  project_id?: string;
  last_sign_in_at: string | null;
  last_call_at: string | null;
  last_served_at: string | null;
  last_outcome?: string;
  calls: number;
  outcomes: Record<string, number>;
  p99_le_ms: number | null;
}

export interface ConfigRow {
  check: string;
  subject: string;
  state: "pass" | "drift" | "info" | "cannot" | "not_needed";
  expected?: unknown;
  found?: unknown;
  facts?: Record<string, unknown>;
}

export interface ClaimsDelivery {
  window: string;
  since: string;
  totals: Record<string, number>;
  findings: ClaimFinding[];
  apps: ClaimApp[];
  sign_ins: {
    checking_since: string;
    last_24h: Record<string, number>;
    waiting: number;
    dropped: number;
    last_sign_in_at: string | null;
  };
  configuration: { checked_at: string; took_ms: number; rows: ConfigRow[] } | null;
  keyring: { read_at: string | null; changed_at: string | null; error?: { status?: number; error?: string }; reads: number };
  alert: { configured: boolean; notes: string[] };
}

export function useClaimsStatus() {
  return useQuery({
    queryKey: ["claims", "status"],
    queryFn: () => request<ClaimsStatus>("/claims/status"),
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
}

export function useClaimsDelivery() {
  return useQuery({
    queryKey: ["claims", "delivery", "24h"],
    queryFn: () => request<ClaimsDelivery>("/claims/delivery?window=24h"),
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
}

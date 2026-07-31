"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Webhook events recorded by the backend for operator visibility — every
 * Zitadel webhook ingest, idempotent-skip included. Mirrors `db.WebhookEvent`.
 */
export interface WebhookEventRow {
  id: string;
  event_type: string;
  user_id: string;
  source_project: string;
  role_key?: string;
  idempotency_key: string;
  status: string;
  error_message?: string;
  created_at: string;
  processed_at?: string | null;
}

/**
 * Onboarding-trigger log row. `source` is "webhook" / "manual" / "system";
 * `bundle_id` is populated only after a welcome bundle resolves.
 */
export interface OnboardingTriggerRow {
  id: string;
  user_id: string;
  source: string;
  idempotency_key: string;
  status: string;
  bundle_id?: string;
  error_message?: string;
  created_at: string;
  completed_at?: string | null;
}

export type OperationsStatus =
  | "pending"
  | "processed"
  | "failed"
  | "completed"
  | "in_flight"
  | "succeeded"
  // An event MkAuth deliberately did not act on — the self-mutation guard
  // dropping its own echo, or an enrichment that never completed. Filterable
  // because "we saw it and chose not to" is a different answer from "we never
  // received it", and only one of them is a bug.
  | "dropped_enrichment_incomplete";

const KEYS = {
  webhookEvents: (status: string) => ["operations", "webhook-events", status] as const,
  onboardingTriggers: ["operations", "onboarding-triggers"] as const,
};

/**
 * Returns the persisted webhook events for the operator queue. Polls on a
 * 5-second cadence so the /operations page reflects newly-ingested events
 * without a manual refresh, suspending while the tab is backgrounded so we
 * don't keep firing requests against an unattended dashboard.
 */
export function useWebhookEvents(filter: { status?: OperationsStatus } = {}) {
  const status = filter.status ?? "";
  return useQuery({
    queryKey: KEYS.webhookEvents(status),
    queryFn: async (): Promise<WebhookEventRow[]> => {
      const path = status
        ? `/webhook/events?status=${encodeURIComponent(status)}`
        : "/webhook/events";
      const data = await request<unknown>(path);
      return Array.isArray(data) ? (data as WebhookEventRow[]) : [];
    },
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
  });
}

/**
 * Returns the onboarding trigger log. The backend returns the full set
 * ordered by created_at desc and does not currently support a status query
 * — client-side filtering covers the four visible buckets.
 */
export function useOnboardingTriggers() {
  return useQuery({
    queryKey: KEYS.onboardingTriggers,
    queryFn: async (): Promise<OnboardingTriggerRow[]> => {
      const data = await request<unknown>("/onboarding/triggers");
      return Array.isArray(data) ? (data as OnboardingTriggerRow[]) : [];
    },
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
  });
}

export const operationsQueryKeys = KEYS;

"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export type IntentStatus = "pending" | "in_flight" | "succeeded" | "failed";

export interface ProvisioningIntent {
  id: string;
  target_uid: string;
  action: string;
  lldap_group: string;
  source_project: string;
  source_role: string;
  webhook_event_id?: string;
  idempotency_key: string;
  status: IntentStatus | string;
  error_message?: string;
  created_at: string;
  acknowledged_at?: string | null;
  completed_at?: string | null;
}

export interface IntentsFilter {
  /** When set, the backend filters server-side via ?status=. */
  status?: IntentStatus;
  /** Client-side cap on render count after fetch. The backend returns the full set. */
  limit?: number;
}

const KEYS = {
  list: (status: string) => ["intents", "list", status] as const,
};

/**
 * Lists provisioning intents. Polls every 5 seconds when mounted because the
 * Operations / "Live operations pulse" surfaces are expected to reflect newly
 * claimed and completed work without a manual refresh.
 */
export function useIntents(filter: IntentsFilter = {}) {
  const status = filter.status ?? "";
  return useQuery({
    queryKey: KEYS.list(status),
    queryFn: async (): Promise<ProvisioningIntent[]> => {
      const path = status ? `/intents?status=${encodeURIComponent(status)}` : "/intents";
      const data = await request<unknown>(path);
      const list = Array.isArray(data) ? (data as ProvisioningIntent[]) : [];
      return filter.limit ? list.slice(0, filter.limit) : list;
    },
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
  });
}

export const intentsQueryKeys = KEYS;

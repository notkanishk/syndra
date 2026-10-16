"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface CascadeSummaryRow {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  source: string;
  source_ref?: string;
  cascade_id?: string;
  status: string;
  completed_at?: string;
  // Set only after a post-write read of Zitadel showed this write actually
  // landed there — "applied" alone means Syndra sent it, not that Zitadel was
  // read back and agreed.
  confirmed_at?: string;
}

/**
 * Change history's unit: every write ONE triggering event produced, collapsed
 * into a single entry. "8 applied", "2 waiting", "no writes" is the whole
 * vocabulary — an entry is a sentence about consequence, not a diff.
 */
export interface CascadeGroupRow {
  cascade_id: string;
  source: string;
  source_ref?: string;
  applied: number;
  waiting: number;
  failed: number;
  // Of `applied`, how many Zitadel has actually been read back and confirmed
  // holds. Always <= applied; the gap is what "sent" and "confirmed" disagree
  // about.
  confirmed: number;
  user_ids: string[];
  writes: CascadeSummaryRow[];
  started_at: string;
  settled_at?: string | null;
}

const KEYS = {
  cascadeGroups: ["propagations", "cascade-groups"] as const,
};

/**
 * Change history. Includes cascades whose writes have NOT landed: a half-applied cascade is the thing that creates
 * unexplained access, and it has to be visible as one.
 */
export function useCascadeGroups(cascadeId?: string) {
  const scope = cascadeId ? `?cascade=${encodeURIComponent(cascadeId)}` : "";
  return useQuery({
    queryKey: [...KEYS.cascadeGroups, cascadeId ?? ""] as const,
    queryFn: async () =>
      (await request<{ cascades: CascadeGroupRow[] }>(`/propagations/cascade-groups${scope}`))
        .cascades ?? [],
  });
}

export const confirmationModeQueryKeys = KEYS;

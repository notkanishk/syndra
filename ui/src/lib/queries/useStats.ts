"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import { invalidateAfter } from "@/lib/queries/invalidation";

/**
 * Mirrors the backend's statistics views (handlers/stats.go). Every day is a
 * makerspace day ("YYYY-MM-DD", the one zone) and every month "YYYY-MM". A
 * series' `days` are sparse: a missing counter day inside an observed window
 * is 0, a missing gauge day is "no new sample", and a gap is never either.
 * Text the add-on wrote (`label`, dimension labels and values, `reason`) is
 * quoted wherever it is shown.
 */

export type StatsUnit = "count" | "items" | "bytes" | "seconds" | "grams";

export interface StatsDay {
  day: string;
  value: number;
}

export interface StatsSeries {
  dimensions: Record<string, string>;
  /** The last 30 makerspace days, oldest first. */
  days: StatsDay[];
  /** A counter's sums over its days and its 12 months. */
  sum_30d?: number;
  sum_12m?: number;
  months?: Array<{ month: string; value: number }>;
  /** A gauge's last sample in the window, with its day. */
  latest?: StatsDay;
}

/** Days nobody could observe; `end` is exclusive (the first day with data again). */
export interface StatsGap {
  start: string;
  end: string;
  dimensions?: Record<string, string>;
  reason: string;
}

export interface StatsCollected {
  at: string | null;
  unbound_accounts?: number;
  dropped?: Record<string, number>;
}

export interface StatsMetric {
  name: string;
  instrument: "counter" | "gauge";
  unit: StatsUnit;
  label: string;
  per_subject?: boolean;
  dimensions?: Array<{ name: string; label: string }>;
  /** A limit field of the same unit: the gauge is a meter over the person's observed limit of it. */
  against?: string;
  /** Null before the metric's first collection. */
  watermark: string | null;
  /** The last makerspace day that is final; later days with no point are not counted yet, never 0. Optional: older data has none. */
  counted_through?: string | null;
  series: StatsSeries[];
  gaps: StatsGap[];
  collected?: StatsCollected;
}

/**
 * Where a page's two windows start, as the backend read them: each window's
 * own first day, or `kept_from` when retention is shorter. Each window is
 * named from its start, never from a window the page works out itself.
 */
export interface UsageWindows {
  /** A person's view only: the first makerspace day whose per-person points are kept on this system; earlier days are gone, not zero. */
  kept_from?: string;
  window_30d_from: string;
  window_12m_from: string;
}

/** Absent: Syndra could not read its own store just now. */
export type Usage = { reported: false } | ({ reported: true; metrics: StatsMetric[] } & UsageWindows);

export interface TargetStats {
  target: string;
  usage: Usage;
  retention: { subject_days: number; set: boolean };
}

export function useTargetStats(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "stats"],
    queryFn: () => request<TargetStats>(`/targets/${encodeURIComponent(target ?? "")}/stats`),
    enabled: Boolean(target),
  });
}

export interface InsightsStatsMetric {
  target: string;
  system: string;
  name: string;
  instrument: "counter" | "gauge";
  unit: StatsUnit;
  label: string;
  per_subject?: boolean;
  against?: string;
  dimensions?: Array<{ name: string; label: string }>;
  /** The first day the spread covers: the window's `from`, or later when that system keeps per-person numbers for fewer days (the backend's own clamp). */
  window_30d_from: string;
  series: Array<{ dimensions: Record<string, string>; members: number; p50: number; p90: number }>;
}

export interface InsightsStats {
  window_days: number;
  from: string;
  metrics: InsightsStatsMetric[];
}

/** Every per-person metric of every system, as spreads over members; operators only, changes nothing. */
export function useInsightsStats() {
  return useQuery({
    queryKey: ["insights", "stats"],
    queryFn: () => request<InsightsStats>("/insights/stats"),
  });
}

/**
 * The one write place for how long per-person numbers are kept (1 to 400
 * days; null is the 400-day ceiling). It deletes older per-person history at
 * the next nightly retention, so the caller asks first when it shortens.
 */
export function useSetStatsRetention(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (days: number | null) =>
      request<{ target: string; subject_days: number; set: boolean }>(`/targets/${encodeURIComponent(target)}/stats-retention`, {
        method: "PUT",
        body: { days },
      }),
    // ["targets", t] reaches this page's stats; every card's "kept from" is in the member and person reads.
    onSuccess: () => invalidateAfter(client, "target", { target, extra: [["me", "targets"], ["users"]] }),
  });
}

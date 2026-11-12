"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

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
  series: StatsSeries[];
  gaps: StatsGap[];
  collected?: StatsCollected;
}

/** Absent: Syndra could not read its own store just now. */
export type Usage = { reported: false } | { reported: true; metrics: StatsMetric[] };

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

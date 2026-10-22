"use client";

import { useQuery } from "@tanstack/react-query";

import { ApiError, request } from "@/lib/api-client";
import type { TopologyGraph } from "@/lib/queries/useTopology";
import type { AccessRoleReason } from "@/lib/queries/useUsers";

/** Mirrors the backend's `Insight*` types. Every read here is operator-only and changes nothing. */

export interface RoleRef {
  project_id: string;
  role_key: string;
}

export interface InsightSingleRoute extends RoleRef {
  source: { kind: "bundle" | "rule"; id: string; name: string };
  only_that_way: number;
  holders: number;
}

export interface InsightOverlap {
  a: { bundle_id: string; name: string };
  b: { bundle_id: string; name: string };
  shared: RoleRef[];
  a_only: RoleRef[];
  b_only: RoleRef[];
}

export interface InsightsFindings {
  single_routes: InsightSingleRoute[];
  unused: {
    roles: RoleRef[];
    bundles: Array<{ bundle_id: string; name: string; holders: number }>;
    projects: Array<{ project_id: string; name: string }>;
  };
  overlaps: InsightOverlap[];
  read_at?: string | null;
}

export interface InsightPerson {
  user_id: string;
  name: string;
  email: string;
  /** How they hold the `has` side: the reasons straight from the access snapshot. */
  how: AccessRoleReason[];
}

export interface InsightWhatIfPerson {
  user_id: string;
  name: string;
  roles: RoleRef[];
}

export interface InsightsWhatIf {
  gains: InsightWhatIfPerson[];
  losses: InsightWhatIfPerson[];
  unchanged: number;
  read_at?: string | null;
}

export interface WhatIfChange {
  kind: "bundle_gives_also" | "bundle_gives_not" | "bundle_gone" | "rule_gone";
  bundle_id?: string;
  project_id?: string;
  role_key?: string;
  rule_id?: string;
}

const list = <T,>(v: T[] | null | undefined): T[] => (Array.isArray(v) ? v : []);

export function useInsightsFindings() {
  return useQuery({
    queryKey: ["insights", "findings"],
    queryFn: async (): Promise<InsightsFindings> => {
      const d = await request<Partial<InsightsFindings>>("/insights/findings");
      return {
        single_routes: list(d?.single_routes),
        unused: {
          roles: list(d?.unused?.roles),
          bundles: list(d?.unused?.bundles),
          projects: list(d?.unused?.projects),
        },
        overlaps: list(d?.overlaps),
        read_at: d?.read_at,
      };
    },
  });
}

/** A ref is `role:<project>:<key>` or `bundle:<id>`: the same string a map node carries. */
export function useInsightsWho(has: string | null, without: string | null) {
  return useQuery({
    queryKey: ["insights", "who", has, without],
    enabled: Boolean(has),
    queryFn: async (): Promise<InsightPerson[]> => {
      const qs = new URLSearchParams({ has: has! });
      if (without) qs.set("without", without);
      const d = await request<{ people?: InsightPerson[] }>(`/insights/who?${qs}`);
      return list(d?.people);
    },
  });
}

/** A POST that only reads: nothing is changed, queued or planned. */
export function useInsightsWhatIf(change: WhatIfChange | null) {
  return useQuery({
    queryKey: ["insights", "whatif", change],
    enabled: Boolean(change),
    queryFn: async (): Promise<InsightsWhatIf> => {
      const d = await request<Partial<InsightsWhatIf>>("/insights/whatif", {
        method: "POST",
        body: { change },
      });
      return { gains: list(d?.gains), losses: list(d?.losses), unchanged: d?.unchanged ?? 0, read_at: d?.read_at };
    },
  });
}

export type TrendPeriod = "30d" | "term" | "year";

export interface TrendWeek {
  week_start: string;
  through_bundle: number;
  by_request: number;
  by_hand: number;
  taken_away: number;
}

export interface InsightsTrends {
  window: { from: string; to: string; label: string };
  weekly: TrendWeek[];
  totals: { given: number; taken_away: number };
  requests: {
    came_in: number;
    approved: number;
    declined: number;
    withdrawn: number;
    declined_reasons: Array<{ reason: string; n: number }>;
    median_hours_to_decide: number | null;
    outside_bundles: Array<RoleRef & { n: number }>;
  };
  sending: { median_minutes_approval_to_sent: number | null; sample: number };
  drift: { found: number; cleared: number; median_hours_to_clear: number | null };
  /** `d14` is optional until the API carries the 14-day count; the page shows a dash without it. */
  ending_soon: Array<{ project_id: string; project_name: string; d14?: number; d30: number; d60: number; d90: number }>;
  upkeep: {
    people_who_left_holding: number;
    drift_open: number;
    roles_nobody_has: number;
    bundles_nothing_new: number;
    overlaps: number;
  };
}

export function useInsightsTrends(period: TrendPeriod) {
  return useQuery({
    queryKey: ["insights", "trends", period],
    queryFn: async (): Promise<InsightsTrends> => {
      const d = await request<InsightsTrends>(`/insights/trends?period=${period}`);
      return {
        ...d,
        weekly: list(d.weekly),
        requests: {
          ...d.requests,
          declined_reasons: list(d.requests?.declined_reasons),
          outside_bundles: list(d.requests?.outside_bundles),
        },
        ending_soon: list(d.ending_soon),
      };
    },
  });
}

/** The graph as Syndra last pictured it, or the date it has none for. */
export type InsightsAsOf =
  | {
      kind: "picture";
      day: string;
      topology: TopologyGraph;
      /** "project_id:role_key" to the people holding it that day. */
      role_people: Record<string, number>;
      first_day: string | null;
      changes_since: number;
    }
  | { kind: "none"; first_day: string | null };

/** "No picture" is an answer, not a failure, so it comes back as data. */
export function useInsightsAsOf(date: string | null) {
  return useQuery({
    queryKey: ["insights", "asof", date],
    enabled: Boolean(date),
    placeholderData: (previous) => previous,
    queryFn: async (): Promise<InsightsAsOf> => {
      try {
        const d = await request<Omit<Extract<InsightsAsOf, { kind: "picture" }>, "kind">>(`/insights/asof?date=${date}`);
        return {
          kind: "picture",
          ...d,
          topology: { nodes: list(d.topology?.nodes), edges: list(d.topology?.edges) },
          role_people: d.role_people ?? {},
        };
      } catch (e) {
        if (e instanceof ApiError && e.code === "NO_SNAPSHOT") {
          return { kind: "none", first_day: (e.details as { first_day?: string | null } | undefined)?.first_day ?? null };
        }
        throw e;
      }
    },
  });
}

export interface InsightsTimeline {
  days: Array<{ day: string; changes: number }>;
  snapshot_days: [string | null, string | null];
}

/** The scrubber's ticks: days with changes, and the first and last day with a picture. */
export function useInsightsTimeline(from: string, to: string, enabled: boolean) {
  return useQuery({
    queryKey: ["insights", "timeline", from, to],
    enabled,
    queryFn: async (): Promise<InsightsTimeline> => {
      const d = await request<Partial<InsightsTimeline>>(`/insights/timeline?from=${from}&to=${to}`);
      return { days: list(d?.days), snapshot_days: d?.snapshot_days ?? [null, null] };
    },
  });
}

/** One role as a lens counts it, named the way a person would say it. */
export interface LensRole extends RoleRef {
  project_name: string;
  role_label: string;
  people: number;
  /** Only the "ending" lens: when the first of them loses it. */
  earliest_end?: string | null;
}

export type LensAsk = { kind: "ending"; days: number } | { kind: "group"; group: string };

export interface InsightsLens {
  kind: "ending" | "group";
  window_days?: number;
  group?: string;
  /** Head count of the group itself, whether or not anything was read. */
  size?: number;
  /** False until Syndra has read who holds what: an empty `roles` then says nothing. */
  holders_observed: boolean;
  roles: LensRole[];
}

/** Ending soon, or what a group holds. Read-only; null asks for nothing. */
export function useInsightsLens(ask: LensAsk | null) {
  return useQuery({
    queryKey: ["insights", "lens", ask],
    enabled: Boolean(ask),
    queryFn: async (): Promise<InsightsLens> => {
      const qs = new URLSearchParams(ask!.kind === "ending" ? { kind: "ending", days: String(ask!.days) } : { kind: "group", group: ask!.group });
      const d = await request<InsightsLens>(`/insights/lens?${qs}`);
      return { ...d, roles: list(d?.roles) };
    },
  });
}

export interface UsageSource {
  receiving: boolean;
  last_at?: string | null;
}

export interface InsightsUsage {
  sources: { sign_in: UsageSource; badge: UsageSource };
  /** Empty, never a guess, unless sign-ins are being received. */
  held_not_used: Array<RoleRef & { people: number }>;
  holders_observed: boolean;
  window_days: number;
}

/** Recent use, from sign-ins. The map's lens and Trends' card read the same answer. */
export function useInsightsUsage(enabled = true) {
  return useQuery({
    queryKey: ["insights", "usage"],
    enabled,
    queryFn: async (): Promise<InsightsUsage> => {
      const d = await request<InsightsUsage>("/insights/usage");
      const src = (x?: UsageSource): UsageSource => ({ receiving: Boolean(x?.receiving), last_at: x?.last_at });
      return {
        sources: { sign_in: src(d?.sources?.sign_in), badge: src(d?.sources?.badge) },
        held_not_used: list(d?.held_not_used),
        holders_observed: Boolean(d?.holders_observed),
        window_days: d?.window_days ?? 90,
      };
    },
  });
}

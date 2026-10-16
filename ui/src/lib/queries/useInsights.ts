"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
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

"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { ObservationBasis } from "@/lib/queries/useRoles";

export interface ProjectSummaryRow {
  project: {
    id: string;
    name: string;
    kind: string;
    description: string;
    roles: Array<{ key: string; label: string }>;
  };
  /** Who Syndra decided belongs to this project — a Recorded fact. */
  member_count: number;
  bundle_count: number;
  rule_in_count: number;
  rule_out_count: number;
  // Roles that exist on this project — the same set the project detail page
  // reads from the role catalog, not roles someone currently holds.
  role_keys: string[];
  sample_members: string[];
  /**
   * How many of member_count the observation store also confirms. Absent —
   * not zero — when `observation.read_at` is missing: see ObservationBasis.
   */
  confirmed_member_count?: number;
  /**
   * One basis for the whole response — identical on every row. Optional so
   * fixtures that don't care about confirmation aren't forced to fabricate
   * one; a missing basis reads the same as "never observed".
   */
  observation?: ObservationBasis;
}

const KEYS = {
  list: ["projects"] as const,
};

/**
 * Fetch the project summary catalog (list + per-project metrics). Powers the
 * /projects page card grid and any place a project picker is needed.
 */
export function useProjects() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: async (): Promise<ProjectSummaryRow[]> => {
      const data = await request<unknown>("/projects");
      return Array.isArray(data) ? (data as ProjectSummaryRow[]) : [];
    },
  });
}

export const projectsQueryKeys = KEYS;

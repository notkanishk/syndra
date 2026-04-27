"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface ProjectSummaryRow {
  project: {
    id: string;
    name: string;
    kind: string;
    description: string;
    roles: Array<{ key: string; label: string }>;
  };
  member_count: number;
  bundle_count: number;
  rule_in_count: number;
  rule_out_count: number;
  active_role_keys: string[];
  sample_members: string[];
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

"use client";

import { useQueryClient } from "@tanstack/react-query";

import type { ApplicationView } from "@/lib/queries/useApplications";
import type { BundleRow } from "@/lib/queries/useBundles";
import type { ProjectSummaryRow } from "@/lib/queries/useProjects";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import type { UserListEntry } from "@/lib/queries/useUsers";

/**
 * The name arrives with you (A-19 P1). A page opened from a row already knows
 * the thing's name: read it from the list the reader came from, else from the
 * name catalog, so the header, avatar and breadcrumb paint on the first frame
 * and only what the row could not know is a skeleton. No request is made here;
 * with nothing cached the answer is undefined and the page behaves as before.
 */
export interface SeededUser {
  name: string;
  email?: string;
  title?: string;
  team?: string;
}

export function useSeededUser(id: string): SeededUser | undefined {
  const qc = useQueryClient();
  const resolver = useNameResolver();
  for (const [, rows] of qc.getQueriesData<UserListEntry[]>({ queryKey: ["users", "list"] })) {
    const hit = rows?.find((row) => row.user.id === id)?.user;
    if (hit?.name) return { name: hit.name, email: hit.email, title: hit.title, team: hit.team };
  }
  const named = resolver.resolveUser(id).value;
  return named?.display_name ? { name: named.display_name, email: named.email } : undefined;
}

/**
 * A project's or bundle's name, from the list the reader came from (the raw
 * cache, so it works before the page's own query has selected anything), else
 * the name catalog. Name only: counts and descriptions stay a skeleton.
 */
export function useSeededProjectName(id: string): string | undefined {
  const qc = useQueryClient();
  const resolver = useNameResolver();
  return (
    qc.getQueryData<ProjectSummaryRow[]>(["projects"])?.find((row) => row.project.id === id)?.project.name ||
    resolver.resolveProject(id).value?.name ||
    undefined
  );
}

export function useSeededBundleName(id: string): string | undefined {
  const qc = useQueryClient();
  const resolver = useNameResolver();
  return (
    qc.getQueryData<BundleRow[]>(["bundles"])?.find((row) => row.id === id)?.name ||
    resolver.resolveBundle(id).value?.name ||
    undefined
  );
}

export function useSeededApp(id: string): ApplicationView["application"] | undefined {
  const qc = useQueryClient();
  return qc.getQueryData<ApplicationView[]>(["applications"])?.find((row) => row.application.id === id)?.application;
}

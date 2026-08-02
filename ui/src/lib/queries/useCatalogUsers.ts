"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { CatalogResponse, ProjectCatalog } from "@/lib/types";

export interface CatalogUser {
  id: string;
  name: string;
  title: string;
  email: string;
}

/**
 * `/catalog` is the one path a member is allowed to read for "what exists
 * here". Both slices below come from the same request and the same cache
 * entry, so the pages that need one or the other make a single network call
 * between them.
 */
const KEY = ["catalog", "response"] as const;

function useCatalog() {
  return useQuery({
    queryKey: KEY,
    queryFn: () => request<CatalogResponse>("/catalog"),
  });
}

/** The persona list for pickers. */
export function useCatalogUsers() {
  const catalog = useCatalog();
  return { ...catalog, data: catalog.data?.users ?? [] };
}

/**
 * Every project and the roles inside it — what a member could ask for.
 *
 * Note this is the `projects` slice, not `applications`. An application is a
 * token consumer (a claim name and a format); nobody requests one. What you
 * ask for is a role in a project, which is what this returns.
 */
export function useCatalogProjects(): {
  data: ProjectCatalog[];
  isLoading: boolean;
  error: unknown;
  refetch: () => void;
} {
  const catalog = useCatalog();
  return {
    data: catalog.data?.projects ?? [],
    isLoading: catalog.isLoading,
    error: catalog.error,
    refetch: catalog.refetch,
  };
}

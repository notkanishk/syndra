"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { Bundle, CatalogResponse } from "@/lib/types";

/**
 * The name catalog and bundle names the NameResolverProvider already holds,
 * read for Go to. `enabled: false` is the point: this subscribes to the same
 * cache entries and never makes a request of its own, so typing costs nothing
 * (the provider warms both on every signed-in page).
 */
export function useNameCatalog() {
  const catalog = useQuery({
    queryKey: ["name-catalog"],
    queryFn: () => request<CatalogResponse>("catalog"),
    enabled: false,
  });
  const bundles = useQuery({
    queryKey: ["name-bundles"],
    queryFn: () => request<Bundle[]>("bundles"),
    enabled: false,
  });
  return { catalog: catalog.data, bundles: bundles.data };
}

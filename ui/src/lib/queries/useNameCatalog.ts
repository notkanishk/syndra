"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { Bundle, CatalogResponse } from "@/lib/types";

/**
 * The name catalog and bundle names the NameResolverProvider already holds,
 * read for Go to. `enabled: false` is the point: this subscribes to the same
 * cache entries and never makes a request of its own, so typing costs nothing
 * (the provider warms both on every signed-in page).
 */
/** The one definition of the two name keys: every reader, writer and invalidation imports them. */
export const NAME_CATALOG_KEY = ["name-catalog"] as const;
export const NAME_BUNDLES_KEY = ["name-bundles"] as const;

/** The name catalog's one query: its key and its fetch, defined once. */
export const nameCatalogQuery = queryOptions({
  queryKey: NAME_CATALOG_KEY,
  queryFn: () => request<CatalogResponse>("catalog"),
});

/** The bundle names' one query. */
export const nameBundlesQuery = queryOptions({
  queryKey: NAME_BUNDLES_KEY,
  queryFn: () => request<Bundle[]>("bundles"),
});

export function useNameCatalog() {
  const catalog = useQuery({ ...nameCatalogQuery, enabled: false });
  const bundles = useQuery({ ...nameBundlesQuery, enabled: false });
  return { catalog: catalog.data, bundles: bundles.data };
}

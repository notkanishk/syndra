import { QueryClient, defaultShouldDehydrateQuery, isServer } from "@tanstack/react-query";
import { cache } from "react";

/**
 * Default options for every QueryClient instance.
 *
 * - staleTime: 30s — most admin views can tolerate this without feeling stale,
 *   and it avoids refetch storms when navigating between pages.
 * - gcTime: 5min — unused query data is dropped after this window so memory
 *   doesn't grow unbounded across long sessions.
 * - retry: 1 — one retry on transient network blips, then surface the error.
 * - refetchOnWindowFocus: false — admin tools are workbench surfaces; tab
 *   focus shouldn't trigger background work.
 *
 * When server-rendering, shouldDehydrateQuery is broadened to include pending
 * queries so streamed data carries through to the client without a re-fetch.
 */
export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        retry: 1,
        refetchOnWindowFocus: false,
      },
      dehydrate: {
        shouldDehydrateQuery: (query) =>
          defaultShouldDehydrateQuery(query) || query.state.status === "pending",
      },
    },
  });
}

let browserQueryClient: QueryClient | undefined;

/**
 * On the server: return a fresh QueryClient per request via React's `cache`
 * so concurrent requests never share mutable state.
 *
 * In the browser: return a single QueryClient for the page lifetime so the
 * cache survives navigations.
 */
export const getQueryClient = cache(function getQueryClientImpl(): QueryClient {
  if (isServer) {
    return makeQueryClient();
  }
  if (!browserQueryClient) {
    browserQueryClient = makeQueryClient();
  }
  return browserQueryClient;
});

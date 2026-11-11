import { MutationCache, QueryClient, isServer } from "@tanstack/react-query";
import { cache } from "react";

import { cooldownOf } from "@/lib/queries/fresh";

/**
 * Default options for every QueryClient instance.
 *
 * - staleTime: 30s — most admin views can tolerate this without feeling stale,
 *   and it avoids refetch storms when navigating between pages.
 * - gcTime: 5min — unused query data is dropped after this window so memory
 *   doesn't grow unbounded across long sessions.
 * - retry: once — one retry on transient network blips (never a cooldown refusal), then surface the error.
 * - refetchOnWindowFocus: false — admin tools are workbench surfaces; tab
 *   focus shouldn't trigger background work.
 */
let lastOwnAct = 0;

/**
 * Whether this person's own write landed in this tab within `ms`. A queue that
 * empties just after one is theirs to celebrate (A-18 idea 5); one that
 * empties on a later read, or arrives empty, is not.
 */
export function ownActWithin(ms: number): boolean {
  return lastOwnAct > 0 && Date.now() - lastOwnAct < ms;
}

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    mutationCache: new MutationCache({ onSuccess: () => void (lastOwnAct = Date.now()) }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        // One retry on a blip; a read refused inside the cooldown is not retried, it would only be refused again.
        retry: (count, error) => count < 1 && !cooldownOf(error),
        refetchOnWindowFocus: false,
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

import type { QueryClient } from "@tanstack/react-query";

/**
 * Seeds the indicators read with the server's answer about its background
 * drain. Without it a screen under a provider knows nothing yet, and since
 * final verification N3 says nothing either way (useAutoRetry is null).
 */
export function seedAutoRetry(client: QueryClient, auto: boolean): QueryClient {
  client.setQueryData(["governance", "indicators"], { auto_retry: auto });
  return client;
}

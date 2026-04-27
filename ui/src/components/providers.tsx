"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { Toaster } from "sonner";

import { ErrorBoundary } from "@/components/ErrorBoundary";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { getQueryClient } from "@/lib/query-client";
import { ThemeProvider } from "@/lib/theme";

/**
 * Single-mount root provider stack for the dashboard. Composes:
 * - QueryClientProvider — TanStack Query (per-page-lifetime cache in browser).
 * - NameResolverProvider — tick-batched UID→name resolver backed by /lookup.
 * - ThemeProvider — data-theme attribute toggling, localStorage persistence.
 * - ErrorBoundary — render-error recovery card so chrome stays alive.
 * - Sonner Toaster — bottom-right toast surface for every mutation outcome.
 *
 * Mounted once in the root layout. Devtools render only in development.
 */
export function Providers({ children }: { children: React.ReactNode }) {
  // The cache() inside getQueryClient ensures one client per request on the
  // server and one shared client for the browser session. Calling it directly
  // here (not in a useState) is correct because Providers is itself rendered
  // once per request on the server and once per page on the client.
  const queryClient = getQueryClient();

  return (
    <QueryClientProvider client={queryClient}>
      <NameResolverProvider>
        <ThemeProvider>
          <ErrorBoundary>{children}</ErrorBoundary>
          <Toaster position="bottom-right" closeButton richColors />
        </ThemeProvider>
      </NameResolverProvider>
      {process.env.NODE_ENV === "development" ? (
        <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
      ) : null}
    </QueryClientProvider>
  );
}

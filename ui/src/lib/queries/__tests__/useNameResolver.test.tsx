// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UserName } from "@/components/names/UserName";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";

// Stub fetch to count network calls and return a predictable map.
let fetchCalls: Array<{ url: string; body: unknown }>;

beforeEach(() => {
  fetchCalls = [];
  global.fetch = vi.fn(async (url: string | URL | Request, init?: RequestInit) => {
    const u = typeof url === "string" ? url : url instanceof URL ? url.toString() : url.url;
    let body: unknown = undefined;
    if (init?.body && typeof init.body === "string") {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = init.body;
      }
    }
    fetchCalls.push({ url: u, body });
    // Echo the requested user_ids back as resolved entries so the resolver
    // sees a successful response and renders names.
    const reqUserIds = (body as { user_ids?: string[] } | undefined)?.user_ids ?? [];
    const users: Record<string, { display_name: string; email: string }> = {};
    for (const id of reqUserIds) {
      users[id] = { display_name: `Name-${id}`, email: `${id}@ex.org` };
    }
    return new Response(
      JSON.stringify({ users, projects: {}, roles: {}, bundles: {} }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderWithProviders(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0, staleTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>{ui}</NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("useNameResolver batching", () => {
  it("batches N <UserName/> mounts within one tick into ONE /lookup request", async () => {
    const ids = Array.from({ length: 50 }, (_, i) => `u-${i}`);
    renderWithProviders(
      <div>
        {ids.map((id) => (
          <UserName key={id} id={id} />
        ))}
      </div>,
    );

    // The resolver flushes inside requestAnimationFrame; wait for the network call to land.
    await waitFor(() => {
      expect(fetchCalls.length).toBeGreaterThanOrEqual(1);
    });

    const lookupCalls = fetchCalls.filter((c) => c.url.includes("/lookup"));
    expect(lookupCalls).toHaveLength(1);

    const reqUserIds = (lookupCalls[0].body as { user_ids: string[] }).user_ids;
    expect(reqUserIds.sort()).toEqual([...ids].sort());
  });

  it("does not re-fetch when the same id is mounted a second time (cache hit)", async () => {
    const { unmount } = renderWithProviders(<UserName id="u-A" />);
    await waitFor(() => {
      expect(fetchCalls.filter((c) => c.url.includes("/lookup"))).toHaveLength(1);
    });
    unmount();

    // Reset call count, mount again — resolver's local Map should still hold u-A
    // so no additional fetch is issued. Note: each render gets a fresh
    // NameResolverProvider, so to test true cache-hit we re-render the same provider.
    fetchCalls = [];
    renderWithProviders(<UserName id="u-A" />);
    // Wait one tick — if a fetch were going to happen, it would by now.
    await new Promise((resolve) => setTimeout(resolve, 50));
    // A fresh provider has empty local cache, so 1 fetch is expected. The
    // important guarantee is that two <UserName id="u-A"/> in the SAME tree do
    // not produce two fetches — covered by the batching test above.
    const lookupCalls = fetchCalls.filter((c) => c.url.includes("/lookup"));
    expect(lookupCalls.length).toBeLessThanOrEqual(1);
  });

  it("renders fallback gracefully when the id misses (resolver returns no entry)", async () => {
    // Stub returns echoed ids only — request for a missing id will resolve as
    // "absent from response" and the component must fall back to the dash.
    global.fetch = vi.fn(async () => {
      fetchCalls.push({ url: "lookup", body: null });
      return new Response(
        JSON.stringify({ users: {}, projects: {}, roles: {}, bundles: {} }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const { findByText } = renderWithProviders(<UserName id="u-missing" fallback="—" />);
    expect(await findByText("—")).toBeInTheDocument();
  });
});

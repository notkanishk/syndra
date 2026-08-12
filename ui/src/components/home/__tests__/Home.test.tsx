// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

// The one property this file is about: a target Syndra cannot read produces no
// findings anywhere else, so every other surface renders a blind week exactly
// like a quiet one. The home page is where that lie would be told out loud.

const state = vi.hoisted(() => ({
  advanced: true,
  governance: {
    pending_requests: [] as unknown[],
    expiring_grants: [] as unknown[],
    cleanup_hints: [] as string[],
    pending_propagation: { count: 0, zitadel_reachable: true },
    drift: { count: 0, top: [] },
    unreconciled_targets: [] as Array<{
      target: string;
      since: string;
      last_seen?: string | null;
      reason?: string;
    }>,
  },
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: state.governance,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  useDecideRequest: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/usePropagation", () => ({
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));

const session = { id: "u1", name: "Ada", email: "ada@example.org" } as unknown as SessionUser;

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>,
  );
}

describe("the home queue and a target nobody can read", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
  });

  it("does not say nothing needs you while a target has not been read", () => {
    state.governance.unreconciled_targets = [
      {
        target: "truenas",
        since: new Date(Date.now() - 7 * 24 * 3600_000).toISOString(),
        reason: "addon truenas: connection refused",
      },
    ];
    renderHome();

    // The failure this guards is not a missing card — it is the sentence that
    // would otherwise sit above one.
    // Both the headline and the calm empty row say it; neither may.
    expect(screen.queryAllByText(/Nothing needs you/i)).toHaveLength(0);
    expect(screen.getByText(/can't vouch for/i)).toBeTruthy();
    // The reason travels: "unreachable" and "answered and refused" send an
    // operator to different machines.
    expect(screen.getByText(/connection refused/)).toBeTruthy();
  });

  it("says nothing needs you when every target has been read", () => {
    renderHome();
    expect(screen.queryAllByText(/Nothing needs you/i).length).toBeGreaterThan(0);
    expect(screen.queryByText(/can't vouch for/i)).toBeNull();
  });
});

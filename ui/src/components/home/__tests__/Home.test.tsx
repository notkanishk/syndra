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
    merge_findings: 0,
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
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [{ target: "truenas", registered: true }], isLoading: false }),
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
    expect(screen.queryAllByText(/Nothing here needs you/i)).toHaveLength(0);
    expect(screen.getByText(/could not check/i)).toBeTruthy();
    // The reason travels: "unreachable" and "answered and refused" send an
    // operator to different machines.
    expect(screen.getByText(/connection refused/)).toBeTruthy();
  });

  it("says nothing needs you when every target has been read", () => {
    renderHome();
    expect(screen.queryAllByText(/Nothing here needs you/i).length).toBeGreaterThan(0);
    expect(screen.queryByText(/could not check/i)).toBeNull();
  });
});

describe("the headline's count is not the nav's count", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
    state.governance.merge_findings = 0;
    state.governance.pending_propagation = { count: 0, zitadel_reachable: true };
  });

  // The collapsed nav button counts PLACES wanting attention, not items — so a
  // headline reading "8 things need you" beside it must say what IT counts too,
  // or the two numbers read as one total that disagrees with itself.
  it("scopes the headline to this page, matching the nav's own disambiguation", () => {
    state.governance.merge_findings = 3;
    renderHome();
    expect(screen.getByText(/Three things here need you\./)).toBeTruthy();
  });
});

describe("pending changes says one true thing, once", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
    state.governance.merge_findings = 0;
  });

  // "Waiting to be sent" and "nothing has changed" used to sit in the same
  // sentence, each denying the other. The reassurance is about Zitadel, not
  // the queue: Zitadel hasn't received any of this yet.
  it("does not say nothing changed right next to a nonzero count", () => {
    state.governance.pending_propagation = { count: 4, zitadel_reachable: true };
    renderHome();
    expect(screen.getByText(/4 changes waiting to be sent to Zitadel/)).toBeTruthy();
    expect(screen.getByText(/none of it has reached Zitadel yet/)).toBeTruthy();
    expect(screen.queryByText(/nothing (there )?has changed/i)).toBeNull();
  });
});

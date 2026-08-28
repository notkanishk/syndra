// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

/**
 * The headline on Today is arithmetic over six counts, and one of them had no
 * block on the page. "Six things need you" over a page carrying five is worse
 * than a missing number: it sends somebody hunting for a screen that is not
 * there, and then leaves them assuming they misread the page.
 */
const state = vi.hoisted(() => ({
  advanced: true,
  findings: 0,
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: {
      pending_requests: [],
      expiring_grants: [],
      cleanup_hints: [],
      pending_propagation: { count: 0, zitadel_reachable: true },
      drift: { count: 0, top: [] },
      unreconciled_targets: [],
      merge_findings: state.findings,
    },
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

beforeEach(() => {
  state.advanced = true;
  state.findings = 0;
});

describe("every count in the Today headline has somewhere to go", () => {
  it("gives merge findings a block of their own", async () => {
    state.findings = 2;
    renderHome();

    expect(await screen.findByText("Waiting on a decision")).toBeTruthy();
    expect(screen.getByText(/2 differences the reconciliation found/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /^Open / })).toBeTruthy();
  });

  // Basic's headline does not count findings, so Basic must not show the block:
  // the count and the block have to agree about which view they are in.
  it("keeps it out of Basic, where the headline does not count it", async () => {
    state.advanced = false;
    state.findings = 2;
    renderHome();

    expect((await screen.findAllByText(/Nothing needs you/)).length).toBeGreaterThan(0);
    expect(screen.queryByText("Waiting on a decision")).toBeNull();
  });

  it("says nothing when there is nothing disputed", async () => {
    renderHome();
    expect((await screen.findAllByText(/Nothing needs you/)).length).toBeGreaterThan(0);
    expect(screen.queryByText("Waiting on a decision")).toBeNull();
  });
});

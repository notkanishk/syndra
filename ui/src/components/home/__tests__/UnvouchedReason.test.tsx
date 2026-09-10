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
const state = vi.hoisted(() => ({ advanced: true, reason: "target_unreachable" }));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: {
      pending_requests: [],
      expiring_grants: [],
      cleanup_hints: [],
      pending_propagation: { count: 0, zitadel_reachable: true },
      drift: { count: 0, top: [] },
      unreconciled_targets: [
        { target: "truenas", since: new Date(Date.now() - 3600_000).toISOString(), reason: state.reason },
      ],
      merge_findings: 0,
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
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({
    data: { welcome_bundle_configured: true, missed: [] },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
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
  state.reason = "target_unreachable";
});


describe("why a system could not be checked", () => {
  it("says it in words, not in the value the row stores", async () => {
    state.reason = "target_unreachable";
    renderHome();

    expect(await screen.findByText(/did not answer when Syndra last tried/i)).toBeTruthy();
    expect(screen.queryByText("target_unreachable")).toBeNull();
  });

  it("tells apart the reason that needs fixing from the one that needs waiting", async () => {
    state.reason = "read_refused";
    renderHome();

    expect(await screen.findByText(/something to fix rather than wait out/i)).toBeTruthy();
  });

  // A reason nobody has worded yet is still more use than none, and it must
  // surface here rather than disappearing quietly.
  it("passes an unrecognised reason through rather than swallowing it", async () => {
    state.reason = "some_new_reason";
    renderHome();

    expect(await screen.findByText("some_new_reason")).toBeTruthy();
  });
});

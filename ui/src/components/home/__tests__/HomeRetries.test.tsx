// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

/**
 * Vocal retries (owner, 5 Nov 2026): a change Zitadel didn't take that Syndra
 * sends again on its own is said on Home, and so is the end of it when Syndra
 * gives up. Neither may go unnoticed or vanish silently.
 */
const state = vi.hoisted(() => ({ pending: 0, retrying: 0, gaveUp: 0 }));

vi.mock("@/lib/queries/useIndicators", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useIndicators")>("@/lib/queries/useIndicators");
  return {
    ...actual,
    useIndicatorsRead: () => ({
      data: {
        pending_requests: 0,
        expiring_grants: 0,
        pending_propagation: state.pending,
        drift: 0,
        unconfirmed_revocations: 0,
        revocations_escalated: false,
        holds_due: 0,
        zitadel_reachable: true,
        retrying: state.retrying,
        gave_up: state.gaveUp,
      },
      isPlaceholderData: false,
    }),
  };
});
vi.mock("@/lib/queries/useServiceAccounts", () => ({
  useServiceAccounts: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  serviceAccountHref: (id: string) => `/system/service-accounts/${id}`,
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: {
      pending_requests: [],
      expiring_grants: [],
      cleanup_hints: [],
      pending_propagation: { count: state.pending, zitadel_reachable: true },
      drift: { count: 0, top: [] },
      unreconciled_targets: [],
      merge_findings: 0,
    },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => {
  const none: never[] = [];
  return {
    useRequestsAdmin: () => ({ data: none, isLoading: false, error: null, refetch: vi.fn() }),
    useDecideRequest: () => ({ mutate: vi.fn(), isPending: false }),
  };
});
vi.mock("@/lib/queries/usePropagation", () => ({
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [], isLoading: false }) }));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({ data: EMPTY_REPORT, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => true,
  useUiView: () => ({ view: "advanced", setView: () => {} }),
}));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));

const EMPTY_REPORT = { welcome_bundle_configured: true, missed: [] };
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
  state.pending = 0;
  state.retrying = 0;
  state.gaveUp = 0;
});

describe("Home says what Syndra is sending again, and what it gave up on", () => {
  it("says how many changes Syndra sends again on its own", async () => {
    state.pending = 3;
    state.retrying = 2;
    renderHome();
    expect(await screen.findByText(/Zitadel didn.t answer for 2 of them\. Syndra sends them again on its own/)).toBeTruthy();
  });

  it("keeps a give-up on Home after the change has left the queue", async () => {
    state.gaveUp = 1;
    renderHome();
    expect(await screen.findByText(/Syndra gave up on one change this week after every try/)).toBeTruthy();
    expect(screen.queryByText(/Nothing here needs you/)).toBeNull();
  });

  it("says nothing about retries when there are none", async () => {
    state.pending = 1;
    renderHome();
    await screen.findByText(/waiting for you to send/);
    expect(screen.queryByText(/sends them again on its own|gave up/)).toBeNull();
  });
});

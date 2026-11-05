// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

/**
 * Vocal retries (owner, 5 Nov 2026): a change Zitadel didn't take that Syndra
 * sends again on its own is said on Home, in Basic as in Advanced, and so is
 * the end of it when Syndra gives up, until someone marks it seen. Neither may
 * go unnoticed or vanish silently.
 */
const state = vi.hoisted(() => ({
  pending: 0, retrying: 0, gaveUp: 0, advanced: true, seen: vi.fn(), stopped: [] as { wait_reason?: string }[],
  auto: true, retryingBy: undefined as Record<string, number> | undefined, gaveUpBy: undefined as Record<string, number> | undefined,
}));

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
        retrying_by: state.retryingBy,
        gave_up_by: state.gaveUpBy,
        auto_retry: state.auto,
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
  useMarkGaveUpSeen: () => ({ mutateAsync: state.seen, isPending: false }),
  useGaveUpPropagations: () => ({ data: state.stopped }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [], isLoading: false }) }));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({ data: EMPTY_REPORT, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
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
  state.stopped = [];
  state.advanced = true;
  state.auto = true;
  state.retryingBy = undefined;
  state.gaveUpBy = undefined;
  state.seen = vi.fn().mockResolvedValue({ status: "seen" });
});

describe("Home says what Syndra is sending again, and what it gave up on", () => {
  it.each([true, false])("says how many changes Syndra sends again on its own (advanced: %s)", async (advanced) => {
    state.advanced = advanced;
    state.pending = 3;
    state.retrying = 2;
    renderHome();
    expect(
      await screen.findByText(/Zitadel didn.t take 2 changes\. Syndra sends them again on its own and says here if it stops\. Nothing for you to do\./),
    ).toBeTruthy();
    // The retrying ones are not "waiting for you to send": only the third is.
    if (advanced) expect(screen.getByText(/1 change is waiting for you to send/)).toBeTruthy();
  });

  it("says why Zitadel refused what Syndra stopped sending, once per cause", async () => {
    state.gaveUp = 3;
    state.stopped = [
      { wait_reason: "refused:not_allowed" },
      { wait_reason: "refused:not_allowed" },
      { wait_reason: undefined },
    ];
    renderHome();
    expect(await screen.findByText(/2 were refused: Syndra's service account isn't allowed to make this change\./)).toBeTruthy();
    expect(screen.queryByText(/One was refused/)).toBeNull();
  });

  it.each([true, false])("keeps a give-up on Home until it is marked seen (advanced: %s)", async (advanced) => {
    state.advanced = advanced;
    state.gaveUp = 1;
    renderHome();
    expect(await screen.findByText(/Syndra stopped sending one change, so it isn.t in Zitadel\..*Then mark this seen\./)).toBeTruthy();
    // Re-audit R11: it points where the change really is, not at Audit.
    const see = screen.getByRole("link", { name: /See it/ });
    expect(see.getAttribute("href")).toBe("/governance/pending?gave_up=1");
    expect(screen.queryByRole("link", { name: /^Audit/ })).toBeNull();
    expect(screen.queryByText(/Nothing here needs you/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Mark seen" }));
    await waitFor(() => expect(state.seen).toHaveBeenCalled());
  });

  // Re-audit R11: a TrueNAS change is said to be TrueNAS's, never Zitadel's.
  it("names the system each change was for", async () => {
    state.retrying = 1;
    state.retryingBy = { truenas: 1 };
    state.gaveUp = 3;
    state.gaveUpBy = { zitadel: 2, truenas: 1 };
    renderHome();
    expect(await screen.findByText(/^TrueNAS didn.t take one change\./)).toBeTruthy();
    expect(screen.getByText(/so they aren.t in TrueNAS and Zitadel\./)).toBeTruthy();
  });

  // With REVOCATION_DRAIN_ENABLED=false nothing is sent again on its own, so
  // Home must not say it is: the step is Send.
  it("says to press Send when this server does not send again on its own", async () => {
    state.retrying = 2;
    state.pending = 2;
    state.auto = false;
    renderHome();
    expect(await screen.findByText(/doesn.t send changes again on its own, so they wait for you\. Press Send on Pending changes to try again\./)).toBeTruthy();
    expect(screen.queryByText(/sends them again on its own and says here/)).toBeNull();
  });

  it("says nothing about retries when there are none", async () => {
    state.pending = 1;
    renderHome();
    await screen.findByText(/waiting for you to send/);
    expect(screen.queryByText(/sends them again on its own|gave up/)).toBeNull();
  });
});

// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

// 3.13: Home decides through the queue's own decide, so an approval here says
// what it says on Requests for each wait_reason — and the row keeps its seat
// to say it, rather than vanishing under the tap.

const state = vi.hoisted(() => ({
  decide: vi.fn(),
  requests: [
    {
      id: "r1",
      requester_id: "u7",
      project_id: "pLaser",
      role_key: "supervisor",
      justification: "Evening inductions.",
      status: "pending",
      created_at: "2026-10-16T06:00:00Z",
    },
  ],
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
      merge_findings: 0,
    },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  // One array, as the query cache gives: a fresh one per render reads as a new list.
  useRequestsAdmin: () => ({ data: state.requests, isLoading: false, error: null, refetch: vi.fn() }),
  useDecideRequest: () => ({ mutateAsync: state.decide, isPending: false }),
}));
vi.mock("@/lib/queries/usePropagation", () => ({
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUserAccess: () => ({ data: undefined }),
}));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [], isLoading: false }),
}));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({
    data: EMPTY_REPORT,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => false }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/requests/Holding", () => ({
  HoldingNote: () => null,
  alreadyHeld: () => false,
}));

// One object, as the query cache gives: a fresh one per render reads as a new list.
const EMPTY_REPORT = { welcome_bundle_configured: true, missed: [] };

const session = { id: "op", name: "Kanishk", email: "k@example.org" } as unknown as SessionUser;

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>,
  );
}

describe("deciding a request on Home", () => {
  it("says what the queue says for a role that waits, in the row's seat", async () => {
    state.decide.mockResolvedValueOnce({ message: "ok", wait_reason: "role_waits" });
    renderHome();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toContain("Approved");
    expect(seat.textContent).toContain("Supervisor always waits for you");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
    // The old text named the requester by id.
    expect(screen.queryByText(/Approved for/)).toBeNull();
  });

  it("says Nothing was sent for a decline", async () => {
    state.decide.mockResolvedValueOnce({ message: "ok" });
    renderHome();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    expect((await screen.findByRole("status")).textContent).toBe("DeclinedNothing was sent");
  });
});

describe("the greeting", () => {
  // The server has its own clock and zone: what it sends must not depend on them,
  // or hydration fails (React #418) and the reader's hour arrives a render late.
  it("sends no hour from the server, only the held seat", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const html = renderToString(
      <QueryClientProvider client={client}>
        <Home session={session} />
      </QueryClientProvider>,
    );
    expect(html).not.toMatch(/>(Morning|Evening)</);
    expect(html).toContain('class="invisible">Afternoon<');
  });

  it("greets by the reader's own hour once it's on their screen", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 9, 20, 20, 0));
    renderHome();
    expect(await screen.findByRole("heading", { level: 1, name: /^Evening/ })).toBeInTheDocument();
    vi.useRealTimers();
  });
});

// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

// 6.71: a decided request on Home keeps its seat for Motion 5b's beat, as it does
// on /requests, and the card does not go with its last row. Under test (reduced
// motion) the outcome shows in place for the beat, then the row is simply gone.

const row = {
  id: "r1",
  requester_id: "u7",
  project_id: "pLaser",
  role_key: "supervisor",
  justification: "Evening inductions.",
  status: "pending",
  created_at: "2026-10-16T06:00:00Z",
};
// One stable array per read, as the query cache gives: a new one every render is a new read.
const state = vi.hoisted(() => ({ decide: vi.fn(), requests: [] as unknown[] }));

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

function Screen() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>
  );
}

const flush = () => act(async () => {});
const tick = (ms: number) => act(() => void vi.advanceTimersByTime(ms));

beforeEach(() => {
  vi.useFakeTimers();
  state.requests = [row];
  state.decide.mockReset().mockResolvedValue({ message: "ok" });
});
afterEach(() => vi.useRealTimers());

describe("a decided request on Home keeps its seat for a beat", () => {
  it("shows Approved after the refetch drops the row, until 900ms have passed", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await flush();

    // The refetch: nothing is pending any more.
    state.requests = [];
    rerender(<Screen />);

    expect(screen.getByRole("status").textContent).toContain("Approved");
    tick(899);
    expect(screen.getByRole("status").textContent).toContain("Approved");
    tick(1);
    expect(screen.queryByText("Approved")).toBeNull();
    expect(screen.queryByText(/Evening inductions/)).toBeNull();
  });

  it("keeps the card, and says nothing needs you only after its last row has left", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await flush();
    state.requests = [];
    rerender(<Screen />);

    expect(screen.getByText("Open requests")).toBeInTheDocument();
    expect(screen.queryByText("Nothing here needs you.")).toBeNull();
    tick(900);
    expect(screen.queryByText("Open requests")).toBeNull();
    expect(screen.getByText("Nothing here needs you.")).toBeInTheDocument();
  });
});

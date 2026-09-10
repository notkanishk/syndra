// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

/**
 * A write Zitadel accepted is not the same fact as a write Zitadel has since
 * shown back — `one-truth-many-checks` gave the gap between them a name and a
 * threshold, and this is the block that makes it visible. It must only ever
 * show what the backend has already decided is old enough to be a finding:
 * the threshold lives once, on the backend, so this page never guesses at its
 * own answer to "how long is too long" — the frontend only renders the count
 * and the list it is handed, and asserts they agree.
 */
const state = vi.hoisted(() => ({
  advanced: true,
  unconfirmed: { count: 0, top: [] as Array<Record<string, unknown>> },
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
      unconfirmed_writes: state.unconfirmed,
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
vi.mock("@/components/names", async () => {
  const actual = await vi.importActual<typeof import("@/components/names")>("@/components/names");
  return {
    ...actual,
    UserName: ({ id }: { id: string | null | undefined }) => <span>{id}</span>,
    UserAvatar: () => <span />,
    RoleRef: ({ roleKey }: { roleKey: string | null | undefined }) => <span>{roleKey}</span>,
  };
});

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
  state.unconfirmed = { count: 0, top: [] };
});

describe("Home shows a write Zitadel accepted and has not confirmed", () => {
  it("stays off the page when nothing is old enough to be a finding — an empty state, not a fault", async () => {
    renderHome();
    await screen.findByText(/last checked/);
    expect(screen.queryByText("Sent, and not seen back yet")).toBeNull();
    expect((await screen.findAllByText(/Nothing here needs you/)).length).toBeGreaterThan(0);
  });

  it("never appears for a write the backend already confirmed — count 0 means no block", async () => {
    // A confirmed write never reaches this summary field at all (the backend
    // only lists rows with confirmed_at IS NULL past the threshold); count 0
    // stands in for that case here since the frontend cannot see confirmed_at
    // directly and must trust the backend's count.
    state.unconfirmed = { count: 0, top: [] };
    renderHome();
    expect(await screen.findByText(/last checked/)).toBeTruthy();
    expect(screen.queryByText("Sent, and not seen back yet")).toBeNull();
  });

  it("says what happened, what it is not, and names the age", async () => {
    state.unconfirmed = {
      count: 1,
      top: [
        {
          id: "outbox-1",
          op_type: "grant",
          user_id: "u-pending",
          project_id: "p1",
          role_keys: ["member"],
          applied_at: new Date(Date.now() - 45 * 60 * 1000).toISOString(),
        },
      ],
    };
    renderHome();

    expect(await screen.findByText("Sent, and not seen back yet")).toBeTruthy();
    // What happened.
    expect(screen.getByText(/Syndra sent this change and Zitadel accepted it/)).toBeTruthy();
    // What it is NOT.
    expect(screen.getByText(/not a failed change/)).toBeTruthy();
    expect(screen.getByText(/not one still waiting to be sent/)).toBeTruthy();
    // The age — the reason it is on screen at all.
    expect(screen.getByText(/45 min ago/)).toBeTruthy();
    // A next move, not a dead end.
    expect(screen.getByRole("link", { name: "Look at their access" })).toBeTruthy();
  });

  it("keeps the header count and the list in agreement", async () => {
    state.unconfirmed = {
      count: 2,
      top: [
        {
          id: "outbox-1",
          op_type: "grant",
          user_id: "u-a",
          project_id: "p1",
          role_keys: ["member"],
          applied_at: new Date().toISOString(),
        },
        {
          id: "outbox-2",
          op_type: "revoke",
          user_id: "u-b",
          project_id: "p1",
          role_keys: ["member"],
          applied_at: new Date().toISOString(),
        },
      ],
    };
    renderHome();

    await screen.findByText("Sent, and not seen back yet");
    expect(screen.getAllByRole("link", { name: "Look at their access" })).toHaveLength(2);
  });

  it("counts toward the headline in Advanced", async () => {
    state.unconfirmed = { count: 1, top: [] };
    renderHome();
    expect(await screen.findByText("One thing here needs you.")).toBeTruthy();
  });

  // Basic's headline does not count it, so Basic must not show the block: the
  // count and the block have to agree about which view they are in.
  it("drops out of Basic entirely", async () => {
    state.unconfirmed = { count: 1, top: [] };
    state.advanced = false;
    renderHome();
    expect((await screen.findAllByText(/Nothing here needs you/)).length).toBeGreaterThan(0);
    expect(screen.queryByText("Sent, and not seen back yet")).toBeNull();
  });
});

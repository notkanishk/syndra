// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

/**
 * Five real accounts joined, got no welcome bundle, and nothing on any screen
 * said so — the trigger row read 'failed' and nobody's queue named a person.
 * This is that block: it must show a config gap as a fact (not a person to
 * chase) and a named miss as a person (not a fault), and it must count toward
 * the headline the same way every other Home block does.
 */
const state = vi.hoisted(() => ({
  advanced: true,
  onboarding: {
    welcome_bundle_configured: true,
    missed: [] as Array<{ user_id: string; name: string; email: string }>,
  },
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
    data: state.onboarding,
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
  state.onboarding = { welcome_bundle_configured: true, missed: [] };
});

describe("Home names the onboarding gap instead of staying silent", () => {
  it("stays out of the queue when everybody active holds the welcome bundle", async () => {
    renderHome();
    await screen.findByText(/last checked/);
    expect(screen.queryByText("New people without a bundle")).toBeNull();
  });

  it("states a missing default as a fact, not a person to chase", async () => {
    state.onboarding = { welcome_bundle_configured: false, missed: [] };
    renderHome();

    expect(await screen.findByText("New people without a bundle")).toBeTruthy();
    expect(screen.getByText(/No default/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /Open bundles/ })).toBeTruthy();
  });

  it("names a person the reconciler found holding nothing", async () => {
    state.onboarding = {
      welcome_bundle_configured: true,
      missed: [{ user_id: "u-silent", name: "Silent Miss", email: "silent@example.edu" }],
    };
    renderHome();

    expect(await screen.findByText("New people without a bundle")).toBeTruthy();
    expect(screen.getByRole("link", { name: "u-silent" })).toBeTruthy();
    expect(screen.getByText("Holds no bundle")).toBeTruthy();
  });

  it("counts the gap in the headline", async () => {
    state.onboarding = { welcome_bundle_configured: false, missed: [] };
    renderHome();

    expect(await screen.findByText("One thing here needs you.")).toBeTruthy();
  });
});

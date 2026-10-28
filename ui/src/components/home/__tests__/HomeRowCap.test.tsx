// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";
import { setMediaQuery } from "@/test-utils/media";

/**
 * A-07 §32–34: every Home block holds at most five rows (three on a phone),
 * newest or soonest first, then one violet "and N more →" to the full list. The
 * block's height has a ceiling, so an intake of 31 or 300 draws the same card.
 */
const ids = (prefix: string, count: number) => Array.from({ length: count }, (_, i) => `${prefix}${i + 1}`);

const state = vi.hoisted(() => ({
  requests: [] as unknown[],
  expiring: [] as unknown[],
  unvouched: [] as unknown[],
  missed: [] as Array<{ user_id: string; name: string; email: string }>,
}));

vi.mock("@/lib/queries/useServiceAccounts", () => ({
  useServiceAccounts: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  serviceAccountHref: (id: string) => `/system/service-accounts/${id}`,
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: {
      pending_requests: [],
      expiring_grants: state.expiring,
      cleanup_hints: [],
      pending_propagation: { count: 0, zitadel_reachable: true },
      drift: { count: 0, top: [] },
      unreconciled_targets: state.unvouched,
      merge_findings: 0,
    },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: state.requests, isLoading: false, error: null, refetch: vi.fn() }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
    data: { welcome_bundle_configured: true, missed: state.missed },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => true }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/requests/Holding", () => ({ HoldingNote: () => null, alreadyHeld: () => false }));
vi.mock("@/components/names", async () => {
  const actual = await vi.importActual<typeof import("@/components/names")>("@/components/names");
  return {
    ...actual,
    UserName: ({ id }: { id: string | null | undefined }) => <span>{id}</span>,
    UserAvatar: () => <span />,
    RoleRef: () => <span>a role</span>,
  };
});

const session = { id: "op", name: "Ada", email: "ada@example.org" } as unknown as SessionUser;

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>,
  );
}

const shown = (prefix: string) => screen.queryAllByText(new RegExp(`^${prefix}\\d+$`)).map((node) => node.textContent);

beforeEach(() => {
  // Requests are listed oldest-first by the read, so "newest first" must be Home's doing.
  state.requests = ids("req", 8).map((id, i) => ({
    id,
    requester_id: id,
    project_id: "pLaser",
    role_key: "supervisor",
    justification: "",
    status: "pending",
    created_at: `2026-10-${String(10 + i).padStart(2, "0")}T06:00:00Z`,
  }));
  // Soonest first: the read hands them latest-first, to prove the ordering is Home's.
  state.expiring = ids("exp", 7)
    .map((id, i) => ({
      id,
      user_id: id,
      project_id: "pLaser",
      role_key: "trained",
      expires_at: `2026-11-${String(1 + i).padStart(2, "0")}T00:00:00Z`,
    }))
    .reverse();
  state.unvouched = [];
  state.missed = ids("new", 9).map((id) => ({ user_id: id, name: id, email: `${id}@example.org` }));
  setMediaQuery("(max-width: 44.99rem)", false);
});

describe("Home blocks hold five rows, then say how many more", () => {
  it("shows five open requests, newest first, and links the rest to Requests", async () => {
    renderHome();
    await screen.findByText(/last checked/);
    expect(shown("req")).toEqual(["req8", "req7", "req6", "req5", "req4"]);
    const more = screen.getByRole("link", { name: "and 3 more" });
    expect(more).toHaveAttribute("href", "/requests");
    expect(more.className).toContain("text-accent-text");
  });

  it("shows five expiring grants, soonest first, and links the rest to Expiring access", async () => {
    renderHome();
    await screen.findByText(/last checked/);
    expect(shown("exp")).toEqual(["exp1", "exp2", "exp3", "exp4", "exp5"]);
    expect(screen.getByRole("link", { name: "and 2 more" })).toHaveAttribute("href", "/review/expiring-access");
  });

  it("opens People with no bundle for the new people beyond five", async () => {
    renderHome();
    await screen.findByText(/last checked/);
    expect(shown("new")).toHaveLength(5);
    expect(screen.getByRole("link", { name: "and 4 more" })).toHaveAttribute("href", "/users?bundle=none");
  });

  it("caps the systems it could not check the same way", async () => {
    state.unvouched = ids("sys", 7).map((target) => ({ target, since: "2026-10-01T00:00:00Z", reason: "" }));
    renderHome();
    await screen.findByText(/last checked/);
    expect(screen.getAllByRole("link", { name: /^Open / })).toHaveLength(5);
  });

  it("draws no line when the block fits", async () => {
    state.requests = (state.requests as unknown[]).slice(0, 5);
    state.expiring = [];
    state.missed = [];
    renderHome();
    await screen.findByText(/last checked/);
    expect(screen.queryByRole("link", { name: /and \d+ more/ })).toBeNull();
    expect(shown("req")).toHaveLength(5);
  });

  it("holds three rows on a phone, and counts what that leaves out", async () => {
    setMediaQuery("(max-width: 44.99rem)", true);
    renderHome();
    await screen.findByText(/last checked/);
    expect(shown("req")).toEqual(["req8", "req7", "req6"]);
    expect(screen.getByRole("link", { name: "and 5 more" })).toHaveAttribute("href", "/requests");
    expect(shown("new")).toHaveLength(3);
    expect(screen.getByRole("link", { name: "and 6 more" })).toHaveAttribute("href", "/users?bundle=none");
  });
});

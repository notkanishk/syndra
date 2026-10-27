// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

// route-home, layout 1c (chosen 2 Oct 2026): "Time-sensitive" on the left, work that
// changes on its own if you wait; "When you're ready" on the right, work that only waits.
// #27: a block with nothing in it gives up its seat, and the line under the headline says
// what was checked and is clear, so a missing block never reads as a missing check.

const state = vi.hoisted(() => ({
  advanced: true,
  requests: [] as unknown[],
  indicators: { unconfirmed_revocations: 0, holds_due: 0 },
  indicatorsReal: true,
  governance: {} as Record<string, unknown>,
  onboarding: { welcome_bundle_configured: true, missed: [] as Array<{ user_id: string }> },
}));

const quiet = () => ({
  pending_requests: [],
  expiring_grants: [] as unknown[],
  cleanup_hints: [],
  pending_propagation: { count: 0, zitadel_reachable: true },
  drift: { count: 0, top: [] },
  unreconciled_targets: [] as unknown[],
  merge_findings: 0,
  unconfirmed_writes: { count: 0, top: [] },
});

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: state.governance,
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
  useTargets: () => ({ data: [{ target: "truenas", registered: true }], isLoading: false }),
}));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({ data: state.onboarding, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicatorsRead: () => ({
    data: state.indicators,
    isPlaceholderData: !state.indicatorsReal,
  }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/requests/Holding", () => ({ HoldingNote: () => null, alreadyHeld: () => false }));

const session = { id: "op", name: "Kanishk", email: "k@example.org" } as unknown as SessionUser;

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>,
  );
}

const request = {
  id: "r1",
  requester_id: "u7",
  project_id: "pLaser",
  role_key: "supervisor",
  justification: "Evening inductions.",
  status: "pending",
  created_at: "2026-10-16T06:00:00Z",
};

const grant = { id: "g1", user_id: "u3", project_id: "pLaser", role_key: "operator", expires_at: "2099-01-01T00:00:00Z" };

function everyBlock() {
  state.requests = [request];
  state.onboarding = { welcome_bundle_configured: true, missed: [{ user_id: "u9" }] };
  state.governance = {
    ...quiet(),
    expiring_grants: [grant],
    pending_propagation: { count: 12, zitadel_reachable: true },
    drift: { count: 4, top: [] },
    unreconciled_targets: [{ target: "truenas", since: "2026-10-01T22:10:00Z", reason: "refused" }],
    merge_findings: 3,
    unconfirmed_writes: { count: 1, top: [{ id: "w1", op_type: "grant", user_id: "u3", sent_at: "2026-10-02T09:12:00Z" }] },
  };
}

beforeEach(() => {
  state.advanced = true;
  state.requests = [];
  state.onboarding = { welcome_bundle_configured: true, missed: [] };
  state.indicators = { unconfirmed_revocations: 0, holds_due: 0 };
  state.indicatorsReal = true;
  state.governance = quiet();
});

describe("Home's two columns", () => {
  it("puts each block type in its column, in the board's order", () => {
    everyBlock();
    renderHome();

    const left = screen.getByRole("region", { name: "Time-sensitive" });
    const right = screen.getByRole("region", { name: "When you're ready" });
    const titles = (column: HTMLElement) =>
      within(column)
        .getAllByTestId("card-count")
        .map((n) => n.previousElementSibling?.textContent);

    expect(titles(left)).toEqual([
      "Drift",
      "Direct access ending soon",
      "Systems Syndra could not check",
      "Sent, and not seen back yet",
    ]);
    expect(titles(right)).toEqual([
      "Open requests",
      "New people without a bundle",
      "Waiting on a decision",
      "Pending changes",
    ]);
  });

  it("says what each column is for", () => {
    everyBlock();
    renderHome();
    expect(screen.getByText("Access ends, stays wrong, or goes unseen if you wait.")).toBeTruthy();
    expect(screen.getByText("Nothing changes until you act.")).toBeTruthy();
  });

  it("a column with nothing in it leaves, and the other keeps its own side", () => {
    state.requests = [request];
    renderHome();

    expect(screen.queryByRole("region", { name: "Time-sensitive" })).toBeNull();
    const right = screen.getByRole("region", { name: "When you're ready" });
    // Not pulled across into the empty column's place: the same seat as when both are full.
    expect(right.closest(".unfold")!.className).toContain("desktop:col-start-2");
  });

  // 5b: a block that empties while you are here is not unmounted; it fades, then its gap closes.
  it("a block that empties while you are here folds instead of vanishing", () => {
    Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
    try {
      everyBlock();
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const home = () => (
        <QueryClientProvider client={client}>
          <Home session={session} />
        </QueryClientProvider>
      );
      const view = render(home());
      expect(screen.getByText("Direct access ending soon")).toBeTruthy();

      state.governance = { ...(state.governance as object), expiring_grants: [] };
      view.rerender(home());

      // Still on screen, still the block it was, closing; the rest of its column has not moved.
      const folding = screen.getByText("Direct access ending soon").closest(".unfold")!;
      expect(folding).toHaveAttribute("data-phase", "closing");
      expect(screen.getByText("Drift")).toBeTruthy();
    } finally {
      delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
    }
  });

  // A-08 §3: one switch moves everything one way. Advanced's blocks open by Unfold, Basic's own
  // sentence folds away, and the blocks both views have hold still.
  it("switching Basic to Advanced opens its blocks by Unfold and folds Basic's footnote", () => {
    Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
    try {
      state.advanced = false;
      state.requests = [request];
      state.governance = { ...quiet(), expiring_grants: [grant], drift: { count: 3, top: [] } };
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const home = () => (
        <QueryClientProvider client={client}>
          <Home session={session} />
        </QueryClientProvider>
      );
      const view = render(home());
      const requests = screen.getByText("Open requests");
      expect(screen.queryByRole("link", { name: "Review them" })).toBeNull();
      const footnote = screen.getByText(/In Basic these blocks are your whole queue/);
      expect(footnote.closest(".unfold")).toHaveAttribute("data-phase", "open");

      state.advanced = true;
      view.rerender(home());

      expect(screen.getByText("Open requests")).toBe(requests);
      expect(screen.getByRole("link", { name: "Review them" }).closest(".unfold")).toHaveAttribute(
        "data-phase",
        "opening",
      );
      // Folding, not gone: it fades first and its gap closes after.
      expect(screen.getByText(/In Basic these blocks are your whole queue/).closest(".unfold")).toHaveAttribute(
        "data-phase",
        "closing",
      );
    } finally {
      delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
    }
  });

  it("Basic: expiring access is time-sensitive, requests wait", () => {
    state.advanced = false;
    state.requests = [request];
    state.governance = { ...quiet(), expiring_grants: [grant] };
    renderHome();

    expect(
      within(screen.getByRole("region", { name: "Time-sensitive" })).getByText("Direct access ending soon"),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("region", { name: "When you're ready" })).getByText("Open requests"),
    ).toBeTruthy();
  });
});

describe("the line under the headline", () => {
  it("names what was checked and is clear, each name a link", () => {
    everyBlock();
    renderHome();

    const line = screen.getByText(/Checked, nothing waiting/).parentElement!;
    expect(within(line).getByRole("link", { name: "Unfinished revocations" }).getAttribute("href")).toBe(
      "/governance/unconfirmed-revocations",
    );
    expect(within(line).getByRole("link", { name: "Holds due" }).getAttribute("href")).toBe("/review/holds");
    // A block that is on the page is not "nothing waiting".
    expect(within(line).queryByRole("link", { name: "Drift" })).toBeNull();
    expect(within(line).queryByRole("link", { name: "Requests" })).toBeNull();
  });

  it("a block that left because it is clear is named instead", () => {
    state.requests = [request];
    renderHome();

    const line = screen.getByText(/Checked, nothing waiting/).parentElement!;
    for (const name of ["Expiring access", "Drift", "Pending changes", "Unfinished revocations", "Holds due"]) {
      expect(within(line).getByRole("link", { name })).toBeTruthy();
    }
    expect(within(line).queryByRole("link", { name: "Requests" })).toBeNull();
  });

  it("never names something that has something waiting", () => {
    everyBlock();
    state.indicators = { unconfirmed_revocations: 2, holds_due: 1 };
    renderHome();
    expect(screen.queryByText(/Checked, nothing waiting/)).toBeNull();
  });

  it("does not claim what it has not read: revocations and holds wait for a real read", () => {
    everyBlock();
    state.indicatorsReal = false;
    renderHome();
    expect(screen.queryByRole("link", { name: "Unfinished revocations" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Holds due" })).toBeNull();
  });

  it("Basic names only the places Basic shows", () => {
    state.advanced = false;
    state.requests = [request];
    renderHome();
    const line = screen.getByText(/Checked, nothing waiting/).parentElement!;
    expect(within(line).getByRole("link", { name: "Expiring access" })).toBeTruthy();
    expect(within(line).queryByRole("link", { name: "Drift" })).toBeNull();
  });
});

describe("the calm line when nothing needs you", () => {
  it("is one line, and no second list of what was checked", () => {
    renderHome();
    expect(screen.getByText("Nothing here needs you.")).toBeTruthy();
    expect(screen.queryByText(/Checked, nothing waiting/)).toBeNull();
    expect(screen.queryByRole("region", { name: "Time-sensitive" })).toBeNull();
    expect(screen.queryByRole("region", { name: "When you're ready" })).toBeNull();
  });
});

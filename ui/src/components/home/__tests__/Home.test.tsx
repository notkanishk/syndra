// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";

// The one property this file is about: a target Syndra cannot read produces no
// findings anywhere else, so every other surface renders a blind week exactly
// like a quiet one. The home page is where that lie would be told out loud.

const state = vi.hoisted(() => ({
  advanced: true,
  every: [] as Array<{ id: string; created_at: string }>,
  pending: [] as Array<{ id: string; created_at: string }>,
  governance: {
    pending_requests: [] as unknown[],
    expiring_grants: [] as unknown[],
    cleanup_hints: [] as string[],
    pending_propagation: { count: 0, zitadel_reachable: true },
    drift: { count: 0, top: [] },
    unreconciled_targets: [] as Array<{
      target: string;
      since: string;
      last_seen?: string | null;
      reason?: string;
    }>,
    merge_findings: 0,
  },
}));

vi.mock("@/lib/queries/useServiceAccounts", () => ({
  useServiceAccounts: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  serviceAccountHref: (id: string) => `/system/service-accounts/${id}`,
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: state.governance,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => {
  // One array, as the query cache gives: a fresh one per render reads as a new list.
  const none: never[] = [];
  return {
    useRequestsAdmin: (status?: string) => ({
      data: status === "all" ? state.every : status === "pending" ? state.pending : none,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    useDecideRequest: () => ({ mutate: vi.fn(), isPending: false }),
  };
});
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
    data: EMPTY_REPORT,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));

// One object, as the query cache gives: a fresh one per render reads as a new list.
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

describe("the home queue and a target nobody can read", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
  });

  it("does not say nothing needs you while a target has not been read", () => {
    state.governance.unreconciled_targets = [
      {
        target: "truenas",
        since: new Date(Date.now() - 7 * 24 * 3600_000).toISOString(),
        reason: "addon truenas: connection refused",
      },
    ];
    renderHome();

    // The failure this guards is not a missing card — it is the sentence that
    // would otherwise sit above one.
    // Both the headline and the calm empty row say it; neither may.
    expect(screen.queryAllByText(/Nothing here needs you/i)).toHaveLength(0);
    expect(screen.getByText(/could not check/i)).toBeTruthy();
    // The reason travels: "unreachable" and "answered and refused" send an
    // operator to different machines.
    expect(screen.getByText(/connection refused/)).toBeTruthy();
    // The claims line holds its seat whether or not anything needs you.
    expect(screen.getByRole("button", { name: "Claims" })).toBeTruthy();
  });

  it("says nothing needs you when every target has been read", () => {
    renderHome();
    expect(screen.queryAllByText(/Nothing here needs you/i).length).toBeGreaterThan(0);
    expect(screen.queryByText(/could not check/i)).toBeNull();
    expect(screen.getByRole("button", { name: "Claims" })).toBeTruthy();
  });
});

describe("the headline's count is not the nav's count", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
    state.governance.merge_findings = 0;
    state.governance.pending_propagation = { count: 0, zitadel_reachable: true };
  });

  // The collapsed nav button counts PLACES wanting attention, not items — so a
  // headline reading "8 things need you" beside it must say what IT counts too,
  // or the two numbers read as one total that disagrees with itself.
  it("scopes the headline to this page, matching the nav's own disambiguation", () => {
    state.governance.merge_findings = 3;
    state.governance.pending_propagation = { count: 2, zitadel_reachable: true };
    renderHome();
    expect(screen.getByText(/Two things need you\./)).toBeTruthy();
  });

  // #31: the headline counts blocks, each badge counts its own noun. The dev box
  // said "98 things need you" over five blocks, eighty-five of those rows in one.
  it("counts blocks, not the rows inside them", () => {
    state.governance.merge_findings = 40;
    state.governance.pending_propagation = { count: 58, zitadel_reachable: true };
    renderHome();
    expect(screen.getByText(/Two things need you\./)).toBeTruthy();
    expect(screen.queryByText(/98/)).toBeNull();
  });
});

describe("pending changes says one true thing, once", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
    state.governance.merge_findings = 0;
  });

  // "Waiting to be sent" and "nothing has changed" used to sit in the same
  // sentence, each denying the other. The reassurance is about Zitadel, not
  // the queue: Zitadel hasn't received any of this yet.
  it("does not say nothing changed right next to a nonzero count", () => {
    state.governance.pending_propagation = { count: 4, zitadel_reachable: true };
    renderHome();
    expect(screen.getByText(/4 changes are waiting for you to send to Zitadel/)).toBeTruthy();
    expect(screen.getByText(/None of it has reached Zitadel yet/)).toBeTruthy();
    expect(screen.queryByText(/nothing (there )?has changed/i)).toBeNull();
  });
});

// S11: Home is the most-visited first paint. The greeting tints in and the blocks arrive after it.
describe("Home's first paint", () => {
  it("wears the screen's arrival on its greeting, and holds the rest in one read gate", () => {
    renderHome();
    const greeting = screen.getByRole("heading", { level: 1 }).parentElement!;
    expect(greeting).toHaveClass("page-head");
    expect(greeting).not.toHaveAttribute("data-settled");
  });
});

describe("quiet hours and the first welcome", () => {
  beforeEach(() => {
    state.advanced = true;
    state.governance.unreconciled_targets = [];
    state.governance.merge_findings = 0;
    state.governance.pending_propagation = { count: 0, zitadel_reachable: true };
  });
  afterEach(() => vi.useRealTimers());

  const at = (hour: number) => vi.useFakeTimers({ toFake: ["Date"], now: new Date(2026, 10, 5, hour, 0, 0) });
  const keeps = "It will keep until morning.";

  it("adds the sentence to the calm panel late at night, once", () => {
    at(23);
    renderHome();
    expect(screen.getByText(/No open requests, and no access ending in the next 14 days\. It will keep until morning\./)).toBeTruthy();
    expect(screen.getAllByText(new RegExp(keeps.replace(/\./g, "\\.")))).toHaveLength(1);
    expect(screen.getByText(/^Evening/)).toBeTruthy();
  });

  it("is not there by day, or at 05:00", () => {
    at(14);
    const { unmount } = renderHome();
    expect(screen.queryByText(/keep until morning/)).toBeNull();
    unmount();
    at(5);
    renderHome();
    expect(screen.queryByText(/keep until morning/)).toBeNull();
  });

  it("is never said when something needs you", () => {
    at(2);
    state.governance.unreconciled_targets = [{ target: "truenas", since: new Date().toISOString(), reason: "refused" }];
    renderHome();
    expect(screen.queryByText(/keep until morning/)).toBeNull();
  });

  const first = { ...session, firstVisit: true } as SessionUser;
  const renderAs = (who: SessionUser) =>
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <Home session={who} />
      </QueryClientProvider>,
    );
  const dateLine = (title: HTMLElement) => title.nextElementSibling;

  it("welcomes on a calm first visit, in the headline's seat, and never otherwise", () => {
    const { unmount } = renderAs(first);
    const title = screen.getByRole("heading", { level: 1 });
    expect(title.textContent).toMatch(/Welcome to Syndra\./);
    expect(title.textContent).not.toMatch(/Nothing needs you/);
    // Nothing was inserted: the date line is still the title's next sibling.
    expect(dateLine(title)?.textContent).toMatch(/\w+day|last checked/);
    unmount();
    renderAs(session);
    expect(screen.queryByText(/Welcome to Syndra/)).toBeNull();
  });

  it("gives the headline to the work on a working day: the work is the welcome", () => {
    state.governance.unreconciled_targets = [{ target: "truenas", since: new Date().toISOString(), reason: "refused" }];
    renderAs(first);
    const title = screen.getByRole("heading", { level: 1 });
    expect(screen.queryByText(/Welcome to Syndra/)).toBeNull();
    expect(title.textContent).toMatch(/needs? you/);
    expect(dateLine(title)?.textContent).toMatch(/\w+day|last checked/);
  });
});

describe("since they were last here (A-20 P18)", () => {
  const renderAs = (who: SessionUser) =>
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <Home session={who} />
      </QueryClientProvider>,
    );
  const daysAgo = (n: number) => new Date(Date.now() - n * 864e5).toISOString();
  const withPrevious = (iso: string) => ({ ...session, previousSignIn: iso }) as SessionUser;
  const weekday = (iso: string) => new Date(iso).toLocaleDateString("en-GB", { weekday: "long" });
  beforeEach(() => {
    state.advanced = true;
    state.every = [];
    state.pending = [];
    state.governance.unreconciled_targets = [];
  });

  it("a calm day says nothing came in, in the panel's own seat", () => {
    const prev = daysAgo(3);
    renderAs(withPrevious(prev));
    expect(screen.getByText(new RegExp(`Since ${weekday(prev)}, nothing came in\\.`))).toBeTruthy();
  });

  it("does not say it when a request came in since", () => {
    const prev = daysAgo(3);
    state.every = [{ id: "r1", created_at: daysAgo(1) }];
    renderAs(withPrevious(prev));
    expect(screen.queryByText(/nothing came in/)).toBeNull();
  });

  it("says nothing without a previous sign-in", () => {
    renderAs(session);
    expect(screen.queryByText(/nothing came in/)).toBeNull();
  });

  it("a working day names how many of the things are new, and never adds a part", () => {
    const prev = daysAgo(3);
    state.pending = [{ id: "r1", created_at: daysAgo(1) }];
    renderAs(withPrevious(prev));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toMatch(
      new RegExp(`One thing needs you, new since ${weekday(prev)}\\.`),
    );
  });

  it("a thing older than the last sign-in is not new", () => {
    state.pending = [{ id: "r1", created_at: daysAgo(5) }];
    renderAs(withPrevious(daysAgo(3)));
    const title = screen.getByRole("heading", { level: 1 }).textContent ?? "";
    expect(title).toMatch(/One thing needs you\./);
    expect(title).not.toMatch(/since/);
  });
});

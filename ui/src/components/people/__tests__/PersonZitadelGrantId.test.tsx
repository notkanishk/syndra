// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

const state = vi.hoisted(() => ({
  advanced: true,
  askedFor: null as string | null,
  grants: [{ id: "zg-77", userId: "u1", projectId: "p1", roleKeys: ["trained"] }],
  error: null as Error | null,
}));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: (userId: string | null) => {
    state.askedFor = userId;
    return {
      data: userId ? { items: state.grants, total: state.grants.length } : { items: [], total: 0 },
      isLoading: false,
      error: state.error,
    };
  },
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", status: "active", avatar: "AL" },
      bundles: [],
      projects: [
        {
          project_id: "p1",
          project_name: "Laser Lab",
          effective_role_keys: ["trained"],
          source_roles: [{ role_key: "trained", reasons: [{ kind: "direct" }] }],
          derived_roles: [],
        },
      ],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson(isOperator: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={isOperator} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.advanced = true;
  state.askedFor = null;
  state.grants = [{ id: "zg-77", userId: "u1", projectId: "p1", roleKeys: ["trained"] }];
  state.error = null;
});

describe("PersonAccess — Zitadel grant id in Advanced (C9a)", () => {
  it("shows the id, labelled as Zitadel's rather than Syndra's", () => {
    renderPerson(true);
    expect(screen.getByText("zg-77")).toBeInTheDocument();
    // The truth is Zitadel: the primary line states what it showed
    // ("In Zitadel"), not the raw id — the id survives as a muted suffix.
    expect(document.body.textContent).toMatch(/In Zitadel/);
  });

  it("stays out of Basic, but the read behind it does not", () => {
    state.advanced = false;
    renderPerson(true);
    expect(screen.queryByText("zg-77")).not.toBeInTheDocument();

    // Basic used to skip the fetch as well, on the reasoning that it should not
    // pay for a panel it does not render. That was true while the answer was
    // only an id to quote in a ticket. It is not an id any more: it is whether
    // the access on this page is real, and a Basic operator was reading rows
    // that stated Syndra's records as though they were Zitadel's.
    expect(state.askedFor).toBe("u1");
  });

  // The route is self-or-operator now: a member reads their OWN observed
  // grants through the same pipe. The raw id stays an operator's handle —
  // gated on `isOperator`, not on the read itself — but the row-level
  // confirmation it feeds is exactly as true for a member as for an operator.
  it("asks on a member's own page too, but keeps the raw id operator-only", () => {
    renderPerson(false);
    expect(state.askedFor).toBe("u1");
    expect(screen.queryByText("zg-77")).not.toBeInTheDocument();
    // The word is introduced with its glossary popover for a member, rather
    // than assumed the way it is for an operator.
    expect(screen.getByRole("button", { name: "Zitadel" })).toBeInTheDocument();
  });

  // Syndra listing roles for a project Zitadel has no grant for is a real condition. Saying
  // "none" names it; a dash would let it read as "not loaded".
  it("says a missing grant is missing, and points at where that gets triaged", () => {
    state.grants = [];
    renderPerson(true);
    expect(document.body.textContent).toMatch(/Not in Zitadel/);
    expect(document.body.textContent).toMatch(/Side by side/);
  });

  it("distinguishes an unreadable Zitadel from an absent grant", () => {
    state.error = new Error("upstream down");
    renderPerson(true);
    expect(document.body.textContent).toMatch(/Zitadel grant · unavailable/);
    expect(document.body.textContent).not.toMatch(/none —/);
  });
});

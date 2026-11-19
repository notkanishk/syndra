// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/** Access over time is an operator's, in Advanced only (A-24 Q11): never read for anyone else. */
const state = vi.hoisted(() => ({ advanced: false, asked: 0 }));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ view: state.advanced ? "advanced" : "basic", setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Becky Lynch", email: "becky@example.edu", title: "", team: "", status: "active", avatar: "BL" },
      bundles: [],
      projects: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
  useAccessHistory: () => {
    state.asked += 1;
    return { data: { since: "2026-01-01", truncated: false, roles: [] }, isPending: false, isError: false, error: null, refetch: () => {} };
  },
}));
vi.mock("@/lib/queries/useAudit", () => ({ useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }) }));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson(isOperator: boolean, advanced: boolean) {
  state.advanced = advanced;
  state.asked = 0;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={isOperator} />
    </QueryClientProvider>,
  );
}

describe("PersonAccess — access over time", () => {
  it("is on an operator's page in Advanced", () => {
    renderPerson(true, true);
    expect(screen.getByRole("group", { name: "Access over time" })).toBeInTheDocument();
    expect(state.asked).toBeGreaterThan(0);
  });

  it("is not in Basic, and is not read there", () => {
    renderPerson(true, false);
    expect(screen.queryByRole("group", { name: "Access over time" })).not.toBeInTheDocument();
    expect(state.asked).toBe(0);
  });

  it("is never a member's, even in Advanced", () => {
    renderPerson(false, true);
    expect(screen.queryByRole("group", { name: "Access over time" })).not.toBeInTheDocument();
    expect(state.asked).toBe(0);
  });
});

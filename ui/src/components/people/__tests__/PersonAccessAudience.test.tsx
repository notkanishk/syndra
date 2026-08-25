// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

const audit = vi.hoisted(() => ({ calls: 0 }));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: {
        id: "u1",
        name: "Ada Lovelace",
        email: "ada@example.edu",
        title: "Student Staff",
        team: "Fabrication",
        status: "active",
        avatar: "AL",
      },
      bundles: [],
      projects: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
}));

// The audit endpoint is operator-gated. Counting calls proves a member's page
// never reaches for it, rather than merely not showing the result.
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => {
    audit.calls += 1;
    return { data: [], isLoading: false, error: null, refetch: () => {} };
  },
}));

vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson(isOperator: boolean) {
  audit.calls = 0;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={isOperator} />
    </QueryClientProvider>,
  );
}

describe("PersonAccess — audience", () => {
  it("gives an operator every tab", () => {
    renderPerson(true);
    expect(screen.getByRole("button", { name: "Access" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Requests" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Activity" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Full audit trail" })).toBeInTheDocument();
  });

  it("never offers a member a control that can only fail", () => {
    // This route deliberately serves a member their own record, but /audit is
    // operator-gated — so an Activity tab or an audit link would put a control
    // on screen whose only possible outcome is a 403.
    renderPerson(false);
    expect(screen.getByRole("button", { name: "Access" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Requests" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Activity" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Full audit trail" })).not.toBeInTheDocument();
    expect(audit.calls).toBe(0);
  });

  it("keeps Requests for a member, because that endpoint does accept self-reads", () => {
    renderPerson(false);
    screen.getByRole("button", { name: "Requests" }).click();
    expect(screen.queryByText(/Couldn’t load/)).not.toBeInTheDocument();
  });
});

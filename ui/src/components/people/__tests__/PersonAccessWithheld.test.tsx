// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

// A role-holder list reads as full access, and a subtractive allowance is
// exactly the case where it is not: the person holds the role, and the
// entitlement that role maps to is being withheld. §6's promise is that "why
// does this person have access to X" has exactly one answer — this is the half
// of the answer that says they do not have it.

const state = vi.hoisted(() => ({
  allowances: [] as Array<Record<string, unknown>>,
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Ada Lovelace", email: "ada@x.edu", status: "active", avatar: "AL" },
      bundles: [],
      projects: [],
      allowances: state.allowances,
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

function band(over: Record<string, unknown> = {}) {
  return {
    id: "a1",
    target: "truenas",
    field: "group",
    value: "lab_makers",
    direction: "deny",
    actor_id: "op_7",
    reason: "safety review",
    in_force: true,
    review_due: false,
    created_at: "2026-01-01T00:00:00Z",
    ...over,
  };
}

function renderPerson() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator />
    </QueryClientProvider>,
  );
}

describe("a hold in force", () => {
  it("says what is withheld, by whom and why", () => {
    state.allowances = [band()];
    renderPerson();

    // The same pill the member reads on their own page — one object, one word
    // for it, so an operator and a member on the phone are discussing the same
    // thing.
    expect(screen.getByText("Withheld")).toBeInTheDocument();
    const row = screen.getByText(/held by op_7/i).closest("li");
    expect(row).not.toBeNull();
    expect(row).toHaveTextContent("safety review");
    expect(row).toHaveTextContent("truenas");
    // The trap, said out loud: holding the role is not holding the access.
    expect(screen.getByText(/still hold a role that maps to it/i)).toBeInTheDocument();
  });

  it("says nothing when every allowance has ended", () => {
    state.allowances = [band({ in_force: false, ended: "2026-02-01T00:00:00Z", ended_by: "op_7" })];
    renderPerson();

    expect(screen.queryByText("Withheld")).not.toBeInTheDocument();
  });
});

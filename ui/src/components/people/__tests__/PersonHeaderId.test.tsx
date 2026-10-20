// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

const view = vi.hoisted(() => ({ advanced: false }));

vi.mock("@/lib/ui-view", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/ui-view")>()),
  useIsAdvanced: () => view.advanced,
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: {
        id: "312458847001600001",
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
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="312458847001600001" isOperator />
    </QueryClientProvider>,
  );
}

// The Basic header is the person's name, email, role and team (route-users-id). The 18-digit
// subject id sat after them, so a screenshot of the page carried an opaque number.
describe("a person's header", () => {
  it("shows no raw id in Basic", () => {
    view.advanced = false;
    renderPerson();
    expect(screen.getByRole("heading", { name: "Ada Lovelace" })).toBeInTheDocument();
    expect(screen.getByText(/ada@example.edu/)).toBeInTheDocument();
    expect(screen.queryByText("312458847001600001")).not.toBeInTheDocument();
  });

  it("keeps the id reachable in Advanced", () => {
    view.advanced = true;
    renderPerson();
    expect(screen.getByText("312458847001600001")).toBeInTheDocument();
  });
});

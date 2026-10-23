// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/**
 * A-07 §28, route-service-accounts: a person's page says what kind of account
 * it is, in Advanced, and is where an operator marks one as a service account.
 */

const state = vi.hoisted(() => ({
  advanced: true,
  user: {} as Record<string, unknown>,
}));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));
vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => ({ data: { items: [], total: 0 }, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: { user: state.user, bundles: [], projects: [], allowances: [], cleanup_hints: [] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
  useUsers: () => ({ data: [], isLoading: false }),
}));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson(isOperator = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={isOperator} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.advanced = true;
  state.user = { id: "u1", name: "Priya Nair", email: "priya@example.edu", title: "", team: "", status: "active", avatar: "PN" };
});
afterEach(cleanup);

describe("This account · Kind", () => {
  it("says a person is a person, and offers Mark as a service account", () => {
    renderPerson();
    const card = screen.getByText("This account").closest(".card") as HTMLElement;

    expect(within(card).getByText("A person")).toBeInTheDocument();
    expect(within(card).getByText("Signs in with a password in Zitadel")).toBeInTheDocument();
    fireEvent.click(within(card).getByRole("button", { name: "Mark as a service account" }));
    expect(screen.getByRole("heading", { name: "Mark Priya Nair as a service account" })).toBeInTheDocument();
  });

  it("is Advanced, and an operator's: Basic and a member never see it", () => {
    state.advanced = false;
    renderPerson();
    expect(screen.queryByText("This account")).not.toBeInTheDocument();

    cleanup();
    state.advanced = true;
    renderPerson(false);
    expect(screen.queryByText("This account")).not.toBeInTheDocument();
  });

  it("links a service account to its own page instead of offering to mark it", () => {
    state.user = { ...state.user, kind: "machine", service_account: true };
    renderPerson();
    const card = screen.getByText("This account").closest(".card") as HTMLElement;

    expect(within(card).getByText("A service account")).toBeInTheDocument();
    expect(within(card).getByRole("link", { name: /Open its service account page/ })).toHaveAttribute(
      "href",
      "/system/service-accounts/u1",
    );
    expect(within(card).queryByRole("button", { name: "Mark as a service account" })).not.toBeInTheDocument();
  });

  it("reads a marked human-type account the same way", () => {
    state.user = { ...state.user, kind: "human", service_account: true };
    renderPerson();
    expect(screen.getByText("A service account")).toBeInTheDocument();
  });
});

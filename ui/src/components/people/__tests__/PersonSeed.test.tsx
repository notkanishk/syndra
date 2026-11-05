// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";
import { PageCrumbProvider, usePageCrumb } from "@/lib/page-crumb";
import type { UserListEntry } from "@/lib/queries/useUsers";

/**
 * A-19 P1: the name arrives with you. A person's page opened from a row paints
 * the row's name, avatar and breadcrumb on the first frame; the read below it
 * is all that waits. And the tab is in the address, so a link keeps it.
 */

const state = vi.hoisted(() => ({ loading: true, tab: null as string | null }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(state.tab ? `tab=${state.tab}` : ""),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: state.loading
      ? undefined
      : {
          user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "", team: "Fabrication", status: "active", avatar: "AL" },
          bundles: [],
          projects: [],
          cleanup_hints: [],
        },
    isLoading: state.loading,
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

const row = {
  user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "Student Staff", team: "Fabrication", status: "active", avatar: "AL" },
} as UserListEntry;

function Crumb() {
  return <p data-testid="crumb">{usePageCrumb().crumb ?? "none"}</p>;
}

function renderPerson(seeded: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (seeded) client.setQueryData(["users", "list", ""], [row]);
  return render(
    <QueryClientProvider client={client}>
      <PageCrumbProvider>
        <Crumb />
        <PersonAccess userId="u1" isOperator />
      </PageCrumbProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.loading = true;
  state.tab = null;
  window.history.replaceState(null, "", "/users/u1");
});

describe("the name arrives with you", () => {
  it("paints the name, the meta and the breadcrumb on the first frame, before any skeleton", () => {
    renderPerson(true);
    // No timers have run: this is the first paint, and the skeleton is 200ms away.
    expect(screen.getByRole("heading", { name: "Ada Lovelace" })).toBeInTheDocument();
    expect(screen.getByText("ada@example.edu")).toBeInTheDocument();
    expect(screen.getByTestId("crumb")).toHaveTextContent("Ada Lovelace");
    expect(screen.queryByLabelText("Loading this person's access")).not.toBeInTheDocument();
  });

  it("covers only the access rows with the skeleton once the read is slow", async () => {
    vi.useFakeTimers();
    try {
      renderPerson(true);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(250);
      });
      expect(screen.getByLabelText("Loading this person's access")).toBeInTheDocument();
      expect(screen.getByRole("heading", { name: "Ada Lovelace" })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("with no row to seed from, behaves as before: no name, no crumb", () => {
    renderPerson(false);
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.getByTestId("crumb")).toHaveTextContent("none");
  });
});

describe("the person's tab is in the address", () => {
  beforeEach(() => {
    state.loading = false;
  });

  it("opens on the tab the link names", () => {
    state.tab = "requests";
    renderPerson(false);
    expect(screen.getByRole("button", { name: "Requests" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Access" })).not.toHaveAttribute("aria-current");
  });

  it("falls back to Access for a tab that is not one", () => {
    state.tab = "nonsense";
    renderPerson(false);
    expect(screen.getByRole("button", { name: "Access" })).toHaveAttribute("aria-current", "page");
  });

  it("writes the tab to the address as a replace, and Access leaves it clean", () => {
    const before = window.history.length;
    renderPerson(false);
    fireEvent.click(screen.getByRole("button", { name: "Activity" }));
    expect(window.location.search).toBe("?tab=activity");
    expect(screen.getByRole("button", { name: "Activity" })).toHaveAttribute("aria-current", "page");
    fireEvent.click(screen.getByRole("button", { name: "Access" }));
    expect(window.location.search).toBe("");
    // The tab is where you are, not somewhere you went: Back is not spent on it.
    expect(window.history.length).toBe(before);
  });
});

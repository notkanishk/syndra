// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PeoplePage from "@/app/users/page";
import type { UserListEntry } from "@/lib/queries/useUsers";

const users = vi.hoisted(() => ({ data: [] as UserListEntry[] }));

vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: () => ({ ...users, isLoading: false, error: null, refetch: () => {} }),
}));

function person(overrides: Partial<UserListEntry> = {}): UserListEntry {
  return {
    user: {
      id: "u1",
      name: "Tomas Beck",
      email: "tomas@example.org",
      title: "Student Staff",
      team: "Fabrication",
      status: "active",
      avatar: "TB",
    },
    bundle_count: 1,
    bundle_names: ["Lab Tech"],
    effective_role_count: 6,
    project_count: 3,
    key_projects: ["Laser Lab"],
    expiring_count: 0,
    open_request_count: 0,
    unexplained_count: 0,
    ...overrides,
  } as UserListEntry;
}

function renderPeople() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PeoplePage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  users.data = [person()];
});

describe("People index", () => {
  it("carries what a person needs, not just what they have", () => {
    users.data = [
      person({ expiring_count: 1, soonest_expiry: new Date(Date.now() + 2 * 86400_000).toISOString() }),
    ];
    renderPeople();
    expect(screen.getByText(/1 expires in 2 days/)).toBeInTheDocument();
  });

  it("shows the most serious signal when a person has more than one", () => {
    users.data = [person({ expiring_count: 1, open_request_count: 1, unexplained_count: 1 })];
    renderPeople();
    // Unexplained access outranks a deadline, which outranks a request.
    expect(screen.getByText("1 unexplained")).toBeInTheDocument();
    expect(screen.queryByText(/open request/)).not.toBeInTheDocument();
  });

  it("renders a dash rather than nothing when a row is clear", () => {
    renderPeople();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("names bundles instead of counting them", () => {
    users.data = [person({ bundle_names: ["Lab Tech", "Studio Member"] })];
    renderPeople();
    expect(screen.getByText("Lab Tech")).toBeInTheDocument();
    expect(screen.getByText("Studio Member")).toBeInTheDocument();
  });

  it("says how much access somebody has in words", () => {
    renderPeople();
    expect(screen.getByText("6 roles across 3 projects")).toBeInTheDocument();
  });

  it("keeps a departed account visible, at reduced contrast", () => {
    users.data = [person({ user: { ...person().user, status: "departed" } })];
    renderPeople();
    const row = screen.getByRole("link", { name: /Tomas Beck/ });
    expect(row.className).toContain("opacity-60");
  });

  it("tells people they can search by role key", () => {
    renderPeople();
    expect(
      screen.getByPlaceholderText("Search name, email or role key…"),
    ).toBeInTheDocument();
  });

  it("paginates explicitly rather than scrolling forever", () => {
    users.data = Array.from({ length: 60 }, (_, index) =>
      person({ user: { ...person().user, id: `u${index}`, name: `Person ${index}` } }),
    );
    renderPeople();
    expect(screen.getByRole("button", { name: /Load next 10/ })).toBeInTheDocument();
  });

  it("counts how many people have access expiring, in the header", () => {
    users.data = [person({ expiring_count: 1 }), person({ user: { ...person().user, id: "u2" } })];
    renderPeople();
    expect(screen.getByText(/1 with access expiring inside 30 days/)).toBeInTheDocument();
  });
});

// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PeoplePage from "@/app/users/page";
import type { UserListEntry } from "@/lib/queries/useUsers";

const users = vi.hoisted(() => ({ data: [] as UserListEntry[] }));
const nav = vi.hoisted(() => ({ url: "/users", replaced: [] as string[] }));
const roleMembers = vi.hoisted(() => ({ data: undefined as { members: Array<{ user: { id: string } }> } | undefined }));

vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: () => ({ ...users, isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "pLaser", name: "Laser Lab", roles: [{ key: "trained", label: "Trained" }] } }],
  }),
}));

vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRoleMembers: () => ({ data: roleMembers.data }),
}));

// The page reads its filters from the URL, so the router stand-in is a plain
// query string the tests set before rendering.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(nav.url.split("?")[1] ?? ""),
  useRouter: () => ({
    replace: (href: string) => {
      nav.replaced.push(href);
      nav.url = href;
    },
  }),
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
    key_project_ids: ["pLaser"],
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
  nav.url = "/users";
  nav.replaced = [];
  roleMembers.data = undefined;
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

describe("People index — filters come from the URL", () => {
  it("narrows to a project by id, so a link survives a rename", () => {
    users.data = [
      person(),
      person({
        user: { ...person().user, id: "u2", name: "Nina Roy" },
        key_projects: ["Wood Shop"],
        key_project_ids: ["pWood"],
      }),
    ];
    nav.url = "/users?project=pLaser";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("narrows to an attention cohort that no column exposes on its own", () => {
    users.data = [
      person({ effective_role_count: 0, project_count: 0 }),
      person({ user: { ...person().user, id: "u2", name: "Nina Roy" } }),
    ];
    nav.url = "/users?attention=no-access";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("counts a departed person as work only while they still hold roles", () => {
    users.data = [
      person({ user: { ...person().user, status: "departed" } }),
      person({
        user: { ...person().user, id: "u2", name: "Nina Roy", status: "departed" },
        effective_role_count: 0,
      }),
    ];
    nav.url = "/users?attention=departed";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    // Cleanly offboarded — not work, and listing it would bury the ones that are.
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("takes role membership from the role endpoint, not from the people list", () => {
    users.data = [
      person(),
      person({ user: { ...person().user, id: "u2", name: "Nina Roy" } }),
    ];
    roleMembers.data = { members: [{ user: { id: "u1" } }] };
    nav.url = "/users?project=pLaser&role=trained";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("shows everyone rather than nobody while role membership is still loading", () => {
    users.data = [person(), person({ user: { ...person().user, id: "u2", name: "Nina Roy" } })];
    roleMembers.data = undefined;
    nav.url = "/users?project=pLaser&role=trained";
    renderPeople();
    // An empty list would read as "nobody holds this", which is a different
    // and much worse claim than "still loading".
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.getByText("Nina Roy")).toBeInTheDocument();
  });
});

describe("People index — bulk mode", () => {
  it("shows no checkboxes at all until bulk mode is on", () => {
    renderPeople();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Bulk actions" })).not.toBeInTheDocument();
  });

  it("keeps the row a link out of bulk mode and a control inside it", () => {
    renderPeople();
    expect(screen.getByRole("link", { name: /Tomas Beck/ })).toBeInTheDocument();

    cleanup();
    nav.url = "/users?bulk=1";
    renderPeople();
    expect(screen.getByRole("checkbox", { name: "Select Tomas Beck" })).toBeInTheDocument();
    // The name is still a way to reach the person — the rest of the row selects.
    expect(screen.getByRole("link", { name: "Tomas Beck" })).toBeInTheDocument();
  });

  it("selects every row matching the filter, not just the rendered page", async () => {
    users.data = Array.from({ length: 60 }, (_, index) =>
      person({ user: { ...person().user, id: `u${index}`, name: `Person ${index}` } }),
    );
    nav.url = "/users?bulk=1";
    renderPeople();

    fireEvent.click(screen.getByRole("checkbox", { name: /Select all 60 people/ }));

    // ...and says so, with a way back to just the visible page: selecting 60
    // when you meant 50 is dozens of people's access.
    expect(await screen.findByText(/All 60 people selected/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /select only the 50 shown/ })).toBeInTheDocument();
  });

  it("drops the selection when the filter changes underneath it", async () => {
    users.data = [person(), person({ user: { ...person().user, id: "u2", name: "Nina Roy" } })];
    nav.url = "/users?bulk=1";
    const { rerender } = renderPeople();

    fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
    expect(await screen.findByText(/1 person selected/)).toBeInTheDocument();

    // A filter change re-aims the action at a different set of people, so the
    // pending selection cannot survive it.
    nav.url = "/users?bulk=1&attention=expiring";
    rerender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <PeoplePage />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(screen.queryByText(/1 person selected/)).not.toBeInTheDocument());
  });

  it("names the filter in the selection bar rather than only counting", async () => {
    nav.url = "/users?bulk=1&project=pLaser";
    renderPeople();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
    expect(await screen.findByText(/in Laser Lab/)).toBeInTheDocument();
  });
});

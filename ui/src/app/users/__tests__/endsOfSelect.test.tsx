// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PeoplePage from "@/app/users/page";
import type { UserListEntry } from "@/lib/queries/useUsers";

/**
 * A-19 P10.2 (Q9): Select is a mode, not structure. It ends when a bulk dialog
 * closes on its result, and only then. P10.5: "Sent, not seen back" opens People
 * filtered to the people it is about.
 */

const users = vi.hoisted(() => ({ data: [] as UserListEntry[] }));
const nav = vi.hoisted(() => ({ url: "/users", replaced: [] as string[] }));

vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: () => ({ ...users, isPlaceholderData: false, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useProjects", () => ({ useProjects: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoleMembers", () => ({ useRoleMembers: () => ({ data: undefined }) }));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(nav.url.split("?")[1] ?? ""),
  useRouter: () => ({
    replace: (href: string) => {
      nav.replaced.push(href);
      nav.url = href;
    },
  }),
}));
// The dialog's own steps are RehearsalDialog's. What matters here is what the page does when
// it closes: with a result (`true`) or without one.
vi.mock("@/components/people/BulkDialog", () => ({
  BulkDialog: ({ onClose }: { onClose: (applied?: boolean) => void }) => (
    <div role="dialog" aria-label="Bulk">
      <button onClick={() => onClose(true)}>Close on the result</button>
      <button onClick={() => onClose(false)}>Close without applying</button>
      <button onClick={() => onClose()}>Close bare</button>
    </div>
  ),
}));

function person(id: string, name: string, over: Partial<UserListEntry> = {}): UserListEntry {
  return {
    user: { id, name, email: `${id}@example.org`, title: "", team: "", status: "active", avatar: "" },
    bundle_count: 1,
    bundle_names: ["Lab Tech"],
    effective_role_count: 1,
    project_count: 1,
    key_projects: [],
    key_project_ids: [],
    expiring_count: 0,
    open_request_count: 0,
    unexplained_count: 0,
    ...over,
  } as UserListEntry;
}

function renderPeople() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <PeoplePage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  users.data = [person("u1", "Kevin Owens"), person("u2", "Amara Osei", { unseen_count: 2 })];
  nav.url = "/users?bulk=1";
  nav.replaced = [];
});

function openBulk() {
  renderPeople();
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Kevin Owens" }));
  fireEvent.click(screen.getByRole("button", { name: "Give a bundle" }));
  expect(screen.getByRole("dialog")).toBeInTheDocument();
}

describe("Select ends with the result (Q9)", () => {
  it("leaves Select when the dialog closes on its result", () => {
    openBulk();
    fireEvent.click(screen.getByRole("button", { name: "Close on the result" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(nav.replaced.at(-1)).toBe("/users");
  });

  it("stays in Select, selection kept, when it closes without a result", () => {
    openBulk();
    fireEvent.click(screen.getByRole("button", { name: "Close without applying" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(nav.replaced).toEqual([]);
    expect(screen.getByRole("checkbox", { name: "Select Kevin Owens" })).toBeChecked();
  });

  it("a bare close is a close without a result", () => {
    openBulk();
    fireEvent.click(screen.getByRole("button", { name: "Close bare" }));
    expect(nav.replaced).toEqual([]);
  });

  it("leaves the filters in force when it ends", () => {
    nav.url = "/users?bulk=1&attention=expiring";
    users.data = [person("u1", "Kevin Owens", { expiring_count: 1 })];
    openBulk();
    fireEvent.click(screen.getByRole("button", { name: "Close on the result" }));
    expect(nav.replaced.at(-1)).toBe("/users?attention=expiring");
  });
});

describe("Sent, not seen back opens People on those people (P10.5)", () => {
  it("narrows to the people with a write that has not been seen back", () => {
    nav.url = "/users?attention=unseen";
    renderPeople();
    expect(screen.getByText("Amara Osei")).toBeInTheDocument();
    expect(screen.queryByText("Kevin Owens")).toBeNull();
  });

  it("without the filter, everyone is listed", () => {
    nav.url = "/users";
    renderPeople();
    expect(screen.getByText("Amara Osei")).toBeInTheDocument();
    expect(screen.getByText("Kevin Owens")).toBeInTheDocument();
  });

  it("a person's row is a link marked as a stop for the arrow keys", () => {
    nav.url = "/users";
    renderPeople();
    const link = screen.getByText("Kevin Owens").closest("a")!;
    expect(link).toHaveAttribute("href", "/users/u1");
    expect(link).toHaveAttribute("data-row");
  });
});

// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import RolesPage from "@/app/roles/page";
import type { CatalogRole } from "@/lib/queries/useRoles";
import { motionInTests } from "@/test-utils/motion";

const state = vi.hoisted(() => ({ roles: [] as unknown[] }));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: state.roles, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/components/roles/CreateRoleDialog", () => ({ CreateRoleDialog: () => null }));

function role(key: string, over: Partial<CatalogRole> = {}): CatalogRole {
  return {
    project_id: "p1",
    project_name: "Laser Lab",
    role_key: key,
    display_name: key,
    description: "",
    group: "Machines",
    bundle_count: 0,
    rule_count: 0,
    assigned_user_count: 1,
    is_unused: false,
    source: "syndra",
    ...over,
  };
}

const headings = () => {
  const row = document.querySelector("[class*='tracking-[0.09em]']") as HTMLElement;
  return Array.from(row.children).map((cell) => (cell.textContent ?? "").replace(/[↑↓]/g, "").trim());
};
const order = () =>
  screen
    .getAllByRole("link")
    .filter((a) => a.getAttribute("href")?.includes("/roles/"))
    .map((a) => ["Alpha", "Beta", "Gamma"].find((n) => a.textContent?.includes(n)));

beforeEach(() => {
  state.roles = [
    role("gamma", { display_name: "Gamma", project_name: "Wood shop", project_id: "p2", assigned_user_count: 5, bundle_count: 2, rule_count: 1 }),
    role("alpha", { display_name: "Alpha", assigned_user_count: 9, waits_for_send: true }),
    role("beta", { display_name: "Beta", project_name: "Studio", project_id: "p3", assigned_user_count: 2, is_unused: true }),
  ];
});

// The board (route-roles): a search, the one-sentence note beside the freshness
// line instead of a bold box, sortable headings, the Waits mark, and Unused said.
describe("the roles index, as drawn", () => {
  it("reads Project, Role, Group, Used by, People", () => {
    render(<RolesPage />);
    expect(headings()).toEqual(["Project", "Role", "Group", "Used by", "People"]);
  });

  it("holds the partial-list note as one quiet sentence, not a boxed lead-in", () => {
    render(<RolesPage />);
    expect(screen.queryByText("This list may be partial.")).not.toBeInTheDocument();
    const note = screen.getByText(/Roles made straight in Zitadel, on a project Syndra cannot read, are not listed/);
    expect(note.tagName).toBe("P");
    expect(note.querySelector("strong")).toBeNull();
    expect(note.className).toMatch(/text-muted/);
    expect(screen.getByRole("link", { name: /Check a project.s roles in Zitadel/ })).toHaveAttribute("href", "/zitadel/projects");
  });

  it("searches by role name, key or project", () => {
    render(<RolesPage />);
    const box = screen.getByRole("textbox", { name: "Search roles by name, key or project" });
    expect(box).toHaveAttribute("placeholder", "Search role, key or project…");
    fireEvent.change(box, { target: { value: "studio" } });
    expect(order()).toEqual(["Beta"]);
    fireEvent.change(box, { target: { value: "ALPHA" } });
    expect(order()).toEqual(["Alpha"]);
    fireEvent.change(box, { target: { value: "wood" } });
    expect(order()).toEqual(["Gamma"]);
    fireEvent.change(box, { target: { value: "zzz" } });
    expect(screen.getByText("No roles match that.")).toBeInTheDocument();
  });

  it("sorts on Project at first, then Role, Group and People on request", () => {
    render(<RolesPage />);
    expect(screen.getByRole("button", { name: "Sort by project" })).toHaveAttribute("data-sort", "asc");
    expect(order()).toEqual(["Alpha", "Beta", "Gamma"]); // Laser Lab, Studio, Wood shop
    fireEvent.click(screen.getByRole("button", { name: "Sort by people" }));
    expect(order()).toEqual(["Beta", "Gamma", "Alpha"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by role" }));
    fireEvent.click(screen.getByRole("button", { name: "Sort by role" }));
    expect(order()).toEqual(["Gamma", "Beta", "Alpha"]);
  });

  it("shows an Unused pill for a role nothing references, and what does for the rest", () => {
    render(<RolesPage />);
    expect(screen.getAllByText("Unused")).toHaveLength(1);
    expect(screen.getByRole("link", { name: /Beta/ }).textContent).toContain("Unused");
    expect(screen.getByText("2 bundles · 1 rule")).toBeInTheDocument();
  });

  it("marks only the role that always waits for you", () => {
    render(<RolesPage />);
    const marked = screen.getAllByRole("img", { name: "Waits for you" });
    expect(marked).toHaveLength(1);
    expect(screen.getByRole("link", { name: /Alpha/ })).toContainElement(marked[0]);
  });

  it("wraps the role, the group and what uses it instead of cutting them", () => {
    render(<RolesPage />);
    const row = screen.getByRole("link", { name: /Gamma/ });
    for (const cell of Array.from(row.children)) {
      expect(cell.className, cell.textContent ?? "").not.toMatch(/truncate/);
    }
    // below desktop Group and Used by fold into a line under the role
    expect(screen.getByText("Machines · 2 bundles · 1 rule")).toBeInTheDocument();
  });

  // Zitadel unreachable: the API sends the project's id, or nothing, for a name
  // it could not read. The id is never the Project column's label; it sits beside the words,
  // in both views, because it is the only thing anyone can look up in Zitadel (A-07 §7).
  it("says Unknown project, with the id beside it and never as the label, when the name could not be read", () => {
    state.roles = [
      role("gamma", { display_name: "Gamma", project_id: "365951028870251526", project_name: "" }),
      role("alpha", { display_name: "Alpha", project_id: "382075332397630470", project_name: "382075332397630470" }),
    ];
    render(<RolesPage />);
    expect(screen.getAllByText("Unknown project")).toHaveLength(2);
    expect(document.body.textContent).toContain("Unknown project 365951028870251526");
    expect(document.body.textContent).toContain("Unknown project 382075332397630470");
  });
});

// A-05 §2.10: a search is not a new list. The roles that still match are the same rows, and the
// ones that do not fade where they were and settle after, never removed under the reader.
describe("the roles index, searched", () => {
  it("fades what no longer matches instead of removing it, and holds what does", () => {
    motionInTests();
    render(<RolesPage />);
    const alpha = screen.getByText("Alpha");
    fireEvent.change(screen.getByRole("textbox", { name: "Search roles by name, key or project" }), {
      target: { value: "alpha" },
    });
    expect(screen.getByText("Alpha")).toBe(alpha);
    expect(alpha.closest(".tint-out")).toBeNull();
    expect(screen.getByText("Beta").closest(".tint-out")).not.toBeNull();
    expect(screen.getByText("Beta").closest(".close-gap")).not.toBeNull();
  });
});

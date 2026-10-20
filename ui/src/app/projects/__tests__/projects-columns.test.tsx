// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ProjectsPage from "@/app/projects/page";
import type { ProjectSummaryRow } from "@/lib/queries/useProjects";

/**
 * The columns of E1, and one defect in particular.
 *
 * "No roles yet — nothing here can be granted" used to be rendered INSIDE the
 * Roles column, which is 60px wide and right-aligned. A 43-character sentence
 * in a 60px box wraps to six lines, and the row grew to four times the height
 * of its neighbours — the whole table read as broken on any deployment holding
 * a project with no roles.
 *
 * The fact is worth saying; the narrow column is not the place to say it. These
 * tests hold both halves: the count column stays a count, and the sentence
 * survives beside the name.
 */

const state = {
  projects: [] as ProjectSummaryRow[],
  apps: [] as Array<{ application: { id: string; name: string; project_id: string } }>,
};

vi.mock("@/lib/queries/useProjects", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useProjects")>(
    "@/lib/queries/useProjects",
  );
  return {
    ...actual,
    useProjects: () => ({
      data: state.projects,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
  };
});

vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: state.apps, isLoading: false, error: null, refetch: vi.fn() }),
}));

function project(
  id: string,
  name: string,
  roles: string[],
  memberCount = 19,
): ProjectSummaryRow {
  return {
    observed_member_count: memberCount,
    project: { id, name, kind: "internal", description: "", roles: [] },
    member_count: memberCount,
    bundle_count: 0,
    rule_in_count: 0,
    rule_out_count: 0,
    role_keys: roles,
    sample_members: [],
  };
}

beforeEach(() => {
  state.projects = [project("p1", "Studio Access", [])];
  state.apps = [];
});

/** The Roles seat: the board's 110px right-aligned cell, desktop only. */
function rolesSeat(row: HTMLElement) {
  return Array.from(row.children).find((child) => child.className.includes("desktop:w-[110px]"));
}

describe("the projects table", () => {
  const SENTENCE = "No roles yet — nothing here can be granted";

  it("still says that nothing in a role-less project can be granted", () => {
    render(<ProjectsPage />);
    expect(screen.getByText(SENTENCE)).toBeTruthy();
  });

  // The regression itself. The sentence must not be a child of the count
  // column, whatever else changes about either.
  it("does not put that sentence inside the 60px count column", () => {
    render(<ProjectsPage />);

    const sentence = screen.getByText(SENTENCE);
    for (let node = sentence as HTMLElement | null; node; node = node.parentElement) {
      expect(
        node.className,
        "the roles column is 60px and right-aligned; a sentence in it wraps to six lines",
      ).not.toMatch(/desktop:w-\[110px\]/);
      if (node.tagName === "A") break;
    }
  });

  it("keeps the count column a count, so the column reads as one", () => {
    render(<ProjectsPage />);

    const row = screen.getByRole("link", { name: /Studio Access/ });
    const roles = rolesSeat(row);

    expect(roles).toBeTruthy();
    expect(roles?.textContent).toBe("0");
  });

  it("puts the sentence beside the name, which is the column with room", () => {
    render(<ProjectsPage />);

    const name = screen.getByText("Studio Access");
    expect(name.parentElement?.textContent).toContain(SENTENCE);
  });

  it("says nothing extra about a project that has roles", () => {
    state.projects = [project("p2", "Badge Reader", ["reader", "admin"])];
    render(<ProjectsPage />);

    expect(screen.queryByText(SENTENCE)).toBeNull();
    const row = screen.getByRole("link", { name: /Badge Reader/ });
    expect(rolesSeat(row)?.textContent).toBe("2");
    expect(screen.getByText("2 roles")).toBeTruthy();
  });

  // The contradiction this file exists to close: a project with roles nobody
  // holds yet is not a project with nothing to grant. member_count is 0 here
  // on purpose — the old count was "roles with a holder", which collapsed
  // this case to the same "0 roles" the sentence guards.
  it("does not say nothing can be granted when a role exists but has no holder", () => {
    state.projects = [project("p3", "Laser Cutter", ["operator"], 0)];
    render(<ProjectsPage />);

    expect(screen.queryByText(SENTENCE)).toBeNull();
    const row = screen.getByRole("link", { name: /Laser Cutter/ });
    expect(rolesSeat(row)?.textContent).toBe("1");
    expect(screen.getByText("1 role")).toBeTruthy();
  });
});

// The board (route-projects): Project, Apps served, Roles, People, the number
// columns last in their 110px seats, search above, the two sortable headings.
describe("the projects board", () => {
  const headings = () => {
    const row = document.querySelector("[class*='tracking-[0.09em]']") as HTMLElement;
    return Array.from(row.children).map((cell) => (cell.textContent ?? "").replace(/[↑↓]/g, "").trim());
  };
  const names = () =>
    screen
      .getAllByRole("link")
      .map((a) => ["Laser Lab", "Wood shop", "Studio"].find((n) => a.textContent?.includes(n)));

  beforeEach(() => {
    state.projects = [
      project("p1", "Wood shop", ["a", "b"], 22),
      project("p2", "Laser Lab", ["a", "b", "c"], 41),
      project("p3", "Studio", ["a"], 9),
    ];
    state.apps = [
      { application: { id: "a1", name: "Laser bookings", project_id: "p2" } },
      { application: { id: "a2", name: "Kiosk", project_id: "p1" } },
    ];
  });

  it("reads Project, Apps served, Roles, People", () => {
    render(<ProjectsPage />);
    expect(headings()).toEqual(["Project", "Apps served", "Roles", "People"]);
  });

  it("lists the apps in the second column, ahead of the counts", () => {
    render(<ProjectsPage />);
    const row = screen.getByRole("link", { name: /Laser Lab/ });
    const cells = Array.from(row.children).map((c) => c.textContent ?? "");
    expect(cells[1]).toBe("Laser bookings");
    expect(cells[2]).toBe("3");
    expect(cells[3]).toMatch(/^41/);
  });

  it("sorts by Project A to Z at first, and by People when asked", () => {
    render(<ProjectsPage />);
    expect(screen.getByRole("button", { name: "Sort by project" })).toHaveAttribute("data-sort", "asc");
    expect(names()).toEqual(["Laser Lab", "Studio", "Wood shop"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by people" }));
    expect(names()).toEqual(["Studio", "Wood shop", "Laser Lab"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by people" }));
    expect(names()).toEqual(["Laser Lab", "Wood shop", "Studio"]);
  });

  it("searches by project name or by the app that reads it", () => {
    render(<ProjectsPage />);
    const box = screen.getByRole("textbox", { name: "Search projects by name or app" });
    expect(box).toHaveAttribute("placeholder", "Search projects or apps…");
    fireEvent.change(box, { target: { value: "kiosk" } });
    expect(names()).toEqual(["Wood shop"]);
    fireEvent.change(box, { target: { value: "stud" } });
    expect(names()).toEqual(["Studio"]);
    fireEvent.change(box, { target: { value: "nothing like it" } });
    expect(screen.getByText("No project matches that.")).toBeTruthy();
  });

  it("says no app reads a project that no app reads", () => {
    render(<ProjectsPage />);
    // Studio has no app; Laser Lab and Wood shop have one each.
    expect(screen.getAllByText("No app reads this yet")).toHaveLength(1);
    expect(screen.getByRole("link", { name: /Studio/ }).textContent).toContain("No app reads this yet");
  });

  it("notes what Zitadel does not show in amber, and wraps the apps", () => {
    state.projects = [{ ...project("p2", "Laser Lab", ["a"], 41), member_count: 43, confirmed_member_count: 41 }];
    render(<ProjectsPage />);
    expect(screen.getByText("2 not in Zitadel yet").className).toMatch(/text-warn-text/);
    const apps = screen.getByText("Laser bookings").parentElement as HTMLElement;
    expect(apps.className).toMatch(/flex-wrap/);
    expect(apps.className).not.toMatch(/truncate/);
  });
});

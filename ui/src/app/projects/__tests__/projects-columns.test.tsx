// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
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

const state = { projects: [] as ProjectSummaryRow[] };

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
  useApplications: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
}));

function project(
  id: string,
  name: string,
  roles: string[],
  memberCount = 19,
): ProjectSummaryRow {
  return {
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
});

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
      ).not.toMatch(/tablet:w-\[60px\]/);
      if (node.tagName === "A") break;
    }
  });

  it("keeps the count column a count, so the column reads as one", () => {
    render(<ProjectsPage />);

    const row = screen.getByRole("link", { name: /Studio Access/ });
    const roles = Array.from(row.children).find((child) =>
      child.className.includes("tablet:w-[60px]"),
    );

    expect(roles).toBeTruthy();
    expect(roles?.textContent).toBe("0 roles");
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
    const roles = Array.from(row.children).find((child) =>
      child.className.includes("tablet:w-[60px]"),
    );
    expect(roles?.textContent).toBe("2 roles");
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
    const roles = Array.from(row.children).find((child) =>
      child.className.includes("tablet:w-[60px]"),
    );
    expect(roles?.textContent).toBe("1 role");
  });
});

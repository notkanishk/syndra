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

function project(id: string, name: string, roles: string[]): ProjectSummaryRow {
  return {
    project: { id, name, kind: "internal", description: "", roles: [] },
    member_count: 19,
    bundle_count: 0,
    rule_in_count: 0,
    rule_out_count: 0,
    active_role_keys: roles,
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
});

// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ProjectsPage from "@/app/projects/page";
import type { ProjectSummaryRow } from "@/lib/queries/useProjects";
import { motionInTests } from "@/test-utils/motion";

const state = { projects: [] as ProjectSummaryRow[] };

vi.mock("@/lib/queries/useProjects", async () => ({
  ...(await vi.importActual<typeof import("@/lib/queries/useProjects")>("@/lib/queries/useProjects")),
  useProjects: () => ({ data: state.projects, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
}));

function project(id: string, name: string): ProjectSummaryRow {
  return {
    observed_member_count: 3,
    project: { id, name, kind: "internal", description: "", roles: [] },
    member_count: 3,
    bundle_count: 0,
    rule_in_count: 0,
    rule_out_count: 0,
    role_keys: ["op"],
    sample_members: [],
  };
}

beforeEach(() => {
  motionInTests();
  state.projects = [project("p1", "Laser Lab"), project("p2", "Wood Shop"), project("p3", "Studio")];
});

/**
 * A-05 §2.10 on a list that filters in the page: the rows that still match are the same rows,
 * the ones that don't fade where they were and then settle, and clearing brings them back in on tint.
 */
describe("Projects, searched", () => {
  const search = (value: string) =>
    fireEvent.change(screen.getByRole("textbox", { name: "Search projects by name or app" }), { target: { value } });

  it("holds the rows that match, fades the others where they were, and opens them back", () => {
    render(<ProjectsPage />);
    const laser = screen.getByText("Laser Lab");

    search("laser");
    expect(screen.getByText("Laser Lab")).toBe(laser);
    expect(laser.closest("[data-leaving]")).toBeNull();
    // Still in the page, fading, and its gap closing: not removed under the reader.
    expect(screen.getByText("Wood Shop").closest("[data-leaving]")).not.toBeNull();
    expect(screen.getByText("Wood Shop").closest("[data-closing]")).not.toBeNull();
    expect(screen.getByText("Wood Shop").closest("[aria-hidden]")).not.toBeNull();

    // Backspaced before it was gone: it turns round from where it is. Same element, no replay.
    const wood = screen.getByText("Wood Shop");
    search("");
    expect(screen.getByText("Laser Lab")).toBe(laser);
    expect(screen.getByText("Wood Shop")).toBe(wood);
    expect(wood.closest("[data-leaving]")).toBeNull();
    expect(wood.closest("[data-fresh]")).toBeNull();
    expect(wood.closest("[data-opens]")).toBeNull();
  });
});

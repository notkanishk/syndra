// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MappingScreen } from "@/components/targets/MappingScreen";
import type { RoleMapping } from "@/lib/queries/useMappings";

/**
 * The mappings screen's header (route-system-targets-id-mappings): a way back
 * to the target, and a subline that says which system, which Zitadel project,
 * how many mappings and which version, in the order the board draws them.
 */

const state: {
  mappings: RoleMapping[];
  version: number;
  project: { id: string; name: string } | null;
} = { mappings: [], version: 4, project: { id: "p_storage", name: "Network storage" } };

vi.mock("@/lib/queries/useMappings", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMappings")>(
    "@/lib/queries/useMappings",
  );
  return {
    ...actual,
    useMappings: () => ({ data: state.mappings, isLoading: false, error: null }),
    useMappingHistory: () => ({
      data: {
        target: "truenas",
        current_version: state.version,
        unpublished: false,
        unpublished_changes: [],
        versions: state.version ? [{ version: state.version, note: "", published_by: "u1", published_at: "2026-08-05T09:00:00Z", entries: [] }] : [],
      },
    }),
  };
});
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [{ target: "truenas", project: state.project }] }),
}));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: () => {} }));
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/VersionBand", () => ({ VersionBand: () => null }));

function mapping(role: string): RoleMapping {
  return { id: role, project_id: "p_storage", role_key: role, field: "group", value: "g" } as RoleMapping;
}

beforeEach(() => {
  state.mappings = [mapping("archive-admin"), mapping("fabrication-lead")];
  state.version = 4;
  state.project = { id: "p_storage", name: "Network storage" };
});

describe("the mappings header", () => {
  it("links back to the target it belongs to", () => {
    render(<MappingScreen target="truenas" />);

    const back = screen.getByRole("link", { name: /TrueNAS/ });
    expect(back.getAttribute("href")).toBe("/system/targets/truenas");
  });

  it("says the system, the project it serves, the mapping count and the version in force", () => {
    render(<MappingScreen target="truenas" />);

    expect(screen.getByText("2 mappings")).toBeTruthy();
    expect(screen.getByText("version 4")).toBeTruthy();
    const project = screen.getByRole("link", { name: "Network storage" });
    expect(project.getAttribute("href")).toBe("/projects/p_storage");
  });

  it("says no mappings, and leaves the version off, before anything is published", () => {
    state.mappings = [];
    state.version = 0;
    render(<MappingScreen target="truenas" />);

    expect(screen.getByText("no mappings")).toBeTruthy();
    expect(screen.queryByText(/version \d/)).toBeNull();
  });

  it("keeps the lede to the two sentences the board draws", () => {
    render(<MappingScreen target="truenas" />);

    expect(
      screen.getByText(/Changing one moves access for all of them, so every change is previewed before anything is sent\./),
    ).toBeTruthy();
  });

  // A project Syndra could not name has an empty name; interpolating it read
  // "holds one of 's roles" and left "Zitadel project" with nothing after it.
  it("never interpolates an empty project name", () => {
    state.project = { id: "p_storage", name: "" };
    render(<MappingScreen target="truenas" />);

    expect(document.body.textContent).not.toMatch(/one of 's roles/);
    expect(screen.getByText(/holds one of the project's roles/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Unknown project" }).getAttribute("href")).toBe("/projects/p_storage");
  });
});

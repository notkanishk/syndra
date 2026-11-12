// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MappingScreen } from "@/components/targets/MappingScreen";
import { ApiError } from "@/lib/api-client";
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
  error: unknown;
} = { mappings: [], version: 4, project: { id: "p_storage", name: "Network storage" }, error: null };

vi.mock("@/lib/queries/useMappings", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMappings")>(
    "@/lib/queries/useMappings",
  );
  return {
    ...actual,
    useMappings: () => ({ data: state.error ? undefined : state.mappings, isLoading: false, error: state.error }),
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
const roster = vi.hoisted(() => ({ loading: false }));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () =>
    roster.loading
      ? { data: undefined, isLoading: true }
      : { data: [{ target: "truenas", name: "TrueNAS", project: state.project }], isLoading: false },
}));
const schema = vi.hoisted(() => ({ read: true }));
vi.mock("@/lib/queries/useFieldSchema", async (orig) => {
  const m = await (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig);
  const { TRUENAS_SCHEMA } = await import("@/lib/__fixtures__/truenas-schema");
  return { ...m, useFieldSchema: () => (schema.read ? TRUENAS_SCHEMA : undefined) };
});
const crumb = vi.hoisted(() => ({ last: undefined as string | null | undefined }));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: (label: string | null | undefined) => void (crumb.last = label) }));
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/VersionBand", () => ({ VersionBand: () => null }));

function mapping(role: string): RoleMapping {
  return { id: role, project_id: "p_storage", role_key: role, field: "group", value: "g" } as RoleMapping;
}

beforeEach(() => {
  state.mappings = [mapping("archive-admin"), mapping("fabrication-lead")];
  state.version = 4;
  state.project = { id: "p_storage", name: "Network storage" };
  state.error = null;
  schema.read = true;
  roster.loading = false;
  crumb.last = undefined;
});

// The system's own crumb waits for the roster's name (crumbsFor), so this page's
// waits too: the trail then grows only at its end, System, then
// System / TrueNAS / Mappings, never System / Mappings with the name put in
// the middle later.
describe("the page's crumb", () => {
  it("is absent while the roster is being read", () => {
    roster.loading = true;
    render(<MappingScreen target="truenas" />);
    expect(crumb.last).toBeNull();
  });

  it("is Mappings once the roster is read", () => {
    render(<MappingScreen target="truenas" />);
    expect(crumb.last).toBe("Mappings");
  });
});

// Review finding 4: the singular was the plural with its "s" cut off ("to
// TrueNAS group"), and "to what TrueNAS give" before the fields were read.
describe("a system with no project yet", () => {
  beforeEach(() => {
    state.error = new ApiError(409, { error: "PROJECT_REQUIRED", message: "Choose it first." });
  });

  it("says one of its groups, by the field's own noun", () => {
    render(<MappingScreen target="truenas" />);
    expect(screen.getByText(/ties one of that project's roles to one group on TrueNAS, so/)).toBeTruthy();
  });

  it("says something on it before its fields are read", () => {
    schema.read = false;
    render(<MappingScreen target="truenas" />);
    expect(screen.getByText(/ties one of that project's roles to something on TrueNAS, so/)).toBeTruthy();
    expect(screen.queryByText(/what TrueNAS give\b/)).toBeNull();
  });
});

describe("the mappings header", () => {
  // The breadcrumb is the way back; the page draws no second one (A-17 §2.24).
  it("draws no hand-made back link above the title", () => {
    render(<MappingScreen target="truenas" />);

    expect(screen.queryByRole("link", { name: /TrueNAS/ })).toBeNull();
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
    expect(screen.getByRole("link", { name: /^Unknown project/ }).getAttribute("href")).toBe("/projects/p_storage");
  });
});

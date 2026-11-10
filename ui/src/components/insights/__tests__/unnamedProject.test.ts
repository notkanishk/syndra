import { describe, expect, it } from "vitest";

import { roleName } from "@/components/insights/lens";
import { buildModel } from "@/components/insights/model";

// The API sends no label for a project it cannot read (one-path W2d; it once
// sent the id, which then led every role in it: "382075332397630470 ·
// Community"). The model is the one place every reader of the map takes its
// names from, so the shared phrase stands in there.
describe("a project the map could not name", () => {
  const graph = {
    nodes: [
      { id: "project:382075332397630470", label: "", kind: "project", project_id: "382075332397630470" },
      { id: "role:382075332397630470:community", label: "Community", kind: "role", project_id: "382075332397630470", meta: { role_key: "community" } },
      { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    ],
    edges: [],
  };

  it("is Unknown project, in its own node and in the roles it holds", () => {
    const m = buildModel(graph as never);
    expect(m.byId.get("project:382075332397630470")?.label).toBe("Unknown project");
    expect(m.nameOf("role:382075332397630470:community")).toBe("Unknown project · Community");
  });

  it("leaves a project that has a name alone", () => {
    expect(buildModel(graph as never).byId.get("project:p1")?.label).toBe("Laser Lab");
  });
});

// A role the map hasn't drawn is said the map's way, "Project · Role", through
// the one fallbacks: an unnamed project, an unlabelled role (its key humanized).
describe("a role the map has not drawn", () => {
  const m = buildModel({ nodes: [], edges: [] } as never);
  it("keeps its own name beside its project's", () => {
    expect(roleName(m, { project_id: "p1", role_key: "laser_trained", project_name: "Laser Lab", role_label: "" })).toBe(
      "Laser Lab · Laser Trained",
    );
  });
  it("says Unknown project when the project has no name", () => {
    expect(roleName(m, { project_id: "382075", role_key: "trained", project_name: "", role_label: "" })).toBe(
      "Unknown project · Trained",
    );
  });
});

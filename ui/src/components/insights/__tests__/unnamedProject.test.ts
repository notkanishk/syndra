import { describe, expect, it } from "vitest";

import { buildModel } from "@/components/insights/model";

// With Zitadel unreachable the API labels a project it cannot read with the
// project's own id, and that id then led every role in it on the map
// ("382075332397630470 · Community"). The model is the one place every reader
// of the map takes its names from, so it is renamed there.
describe("a project the map could not name", () => {
  const graph = {
    nodes: [
      { id: "project:382075332397630470", label: "382075332397630470", kind: "project", project_id: "382075332397630470" },
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

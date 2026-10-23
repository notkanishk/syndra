// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LaneMap } from "@/components/insights/LaneMap";
import { buildModel, type Highlight } from "@/components/insights/model";

const model = buildModel({
  nodes: [
    { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    { id: "role:p1:op", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "op" } },
  ],
  edges: [{ id: "e1", source: "project:p1", target: "role:p1:op", kind: "contains", label: "" }],
});
const lit: Highlight = {
  nodes: new Map([["role:p1:op", "accent"]]),
  threads: new Map([["e1", "accent"]]),
};

function map(props: { answered?: boolean; breathing?: string | null }) {
  return render(
    <LaneMap
      model={model}
      highlight={lit}
      focusId={null}
      count={() => null}
      unused={new Set()}
      marks={new Map()}
      keysOpen={false}
      onKeys={() => {}}
      onPick={() => {}}
      {...props}
    />,
  );
}

/** Motion v2 (Insights): the answer's lights travel once and rest; a hover lights its line on tint. */
describe("LaneMap motion", () => {
  it("an answer's lit threads travel once", () => {
    const { container } = map({ answered: true });
    expect(container.querySelector("svg g.light-sweep")).not.toBeNull();
    expect(container.querySelector("svg g.tint-in")).toBeNull();
  });

  it("a line lit by hovering or following tints in, it does not travel", () => {
    const { container } = map({ answered: false });
    expect(container.querySelector("svg g.tint-in")).not.toBeNull();
    expect(container.querySelector("svg g.light-sweep")).toBeNull();
  });

  it("only the asked node breathes while its answer is computed", () => {
    map({ breathing: "role:p1:op" });
    expect(screen.getByRole("button", { name: "Laser Lab · Operator" })).toHaveClass("breathe");
    expect(screen.getByRole("button", { name: "Laser Lab" })).not.toHaveClass("breathe");
  });

  it("nothing breathes at rest", () => {
    const { container } = map({});
    expect(container.querySelector(".breathe")).toBeNull();
  });
});

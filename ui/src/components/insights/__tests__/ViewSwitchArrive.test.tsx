// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LaneMap } from "@/components/insights/LaneMap";
import { buildModel } from "@/components/insights/model";
import { NodeList } from "@/components/insights/NodeList";

const model = buildModel({
  nodes: [
    { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    { id: "role:p1:op", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "op" } },
  ],
  edges: [{ id: "e1", source: "project:p1", target: "role:p1:op", kind: "contains", label: "" }],
});

const common = { model, highlight: null, focusId: null, unused: new Set<string>(), onPick: () => {} };

/**
 * Spec S10: the page arrives once. Switching between Map and List swaps the view and the
 * 8px cascade never replays over about forty nodes.
 */
describe("Insights Map and List", () => {
  it("the first view arrives: its lanes have not settled", () => {
    const list = render(<NodeList {...common} />);
    expect(list.container.querySelectorAll(".arrive-list").length).toBeGreaterThan(0);
    expect(list.container.querySelector(".arrive-list[data-settled]")).toBeNull();
    list.unmount();

    const map = render(
      <LaneMap {...common} count={() => null} marks={new Map()} keysOpen={false} onKeys={() => {}} />,
    );
    expect(map.container.querySelector(".arrive-list")).not.toBeNull();
    expect(map.container.querySelector(".arrive-list[data-settled]")).toBeNull();
  });

  it("a view switched to is simply there: every lane starts settled", () => {
    const list = render(<NodeList {...common} startSettled />);
    expect(list.container.querySelector(".arrive-list:not([data-settled])")).toBeNull();
    expect(list.container.querySelectorAll(".arrive-list[data-settled]").length).toBeGreaterThan(0);
    list.unmount();

    const map = render(
      <LaneMap {...common} count={() => null} marks={new Map()} keysOpen={false} onKeys={() => {}} startSettled />,
    );
    expect(map.container.querySelector(".arrive-list:not([data-settled])")).toBeNull();
    expect(map.container.querySelectorAll(".arrive-list[data-settled]").length).toBeGreaterThan(0);
  });
});

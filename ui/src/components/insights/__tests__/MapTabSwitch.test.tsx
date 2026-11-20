// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DENSE_FIELD } from "@/components/ui/Input";
import { setMediaQuery } from "@/test-utils/media";

let query = "";
vi.mock("next/navigation", () => ({
  usePathname: () => "/insights",
  useRouter: () => ({ replace: () => {} }),
  useSearchParams: () => new URLSearchParams(query),
}));

const graph = {
  nodes: [
    { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    { id: "role:p1:op", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "op" } },
  ],
  edges: [{ id: "e1", source: "project:p1", target: "role:p1:op", kind: "contains", label: "" }],
};
vi.mock("@/lib/queries/useTopology", () => ({
  useTopology: () => ({ data: graph, dataUpdatedAt: 0, isLoading: false, isError: false }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogUsers: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useInsights", () => ({
  useInsightsAsOf: () => ({ data: undefined, isError: false }),
  useInsightsLens: () => ({ data: undefined }),
  useInsightsTimeline: () => ({ data: undefined }),
  useInsightsUsage: () => ({ data: undefined }),
}));
vi.mock("@/components/insights/useAnswer", () => ({ useAnswer: () => ({ answer: null, status: null, pin: null }) }));
vi.mock("@/components/insights/useFindings", () => ({
  useFindings: () => ({
    ready: true,
    rest: [],
    counts: { drift: 0, send: 0, single: 0, unused: 0, overlap: 0 },
    held: new Map(),
    unused: new Set(),
    marks: new Map(),
    lit: () => ({ nodes: [], threads: [] }),
    panel: () => null,
    glance: { tone: "neutral", eyebrow: "At a glance", title: "Glance", lines: [] },
  }),
}));

import { MapTab } from "@/components/insights/MapTab";
import { useInsightsUrl } from "@/components/insights/useInsightsUrl";

function Mount() {
  return <MapTab url={useInsightsUrl()} />;
}

/** Spec S10: the page arrives once; Map and List swap with nothing replaying. */
describe("Insights Map and List switch", () => {
  beforeEach(() => {
    query = "";
    setMediaQuery("(max-width: 44.99rem)", false);
    setMediaQuery("(max-width: 67.49rem)", false);
  });

  // The lanes of the view itself; the outer wrapper is the page's own arrival.
  const lanes = (root: HTMLElement) => Array.from(root.querySelectorAll<HTMLElement>(".arrive-list")).slice(1);

  it("the first view arrives, and the one switched to does not", () => {
    const { container, rerender } = render(<Mount />);
    expect(lanes(container).length).toBeGreaterThan(0);
    expect(lanes(container).every((lane) => !lane.hasAttribute("data-settled"))).toBe(true);

    query = "view=list";
    rerender(<Mount />);
    expect(lanes(container).length).toBeGreaterThan(0);
    expect(lanes(container).every((lane) => lane.hasAttribute("data-settled"))).toBe(true);

    // And back: still nothing replays.
    query = "";
    rerender(<Mount />);
    expect(lanes(container).every((lane) => lane.hasAttribute("data-settled"))).toBe(true);
  });
});

/**
 * The toolbar's first row is the question, the lens pills and the view switch,
 * whatever lens is chosen. A lens's own choice is a line beneath it: in the row,
 * its arrival pushed the pills you had just clicked sideways (owner, 20 Nov 2026).
 */
describe("Insights toolbar", () => {
  beforeEach(() => {
    setMediaQuery("(max-width: 44.99rem)", false);
    setMediaQuery("(max-width: 67.49rem)", false);
  });

  it.each([
    ["lens=ending", () => screen.getByRole("radiogroup", { name: "How far ahead" })],
    ["lens=group", () => screen.getByRole("combobox", { name: "Which group" })],
    ["lens=group&group=team%3Alasers", () => screen.getByRole("textbox", { name: "Team name" })],
  ])("keeps %s's choice out of the first row, on a line of its own beneath it", (q, choice) => {
    query = q;
    const { container } = render(<Mount />);
    const row = container.querySelector<HTMLElement>("[data-toolbar]")!;
    expect(within(row).getByRole("radiogroup", { name: "Lens" })).toBeInTheDocument();
    expect(within(row).getByRole("radiogroup", { name: "View" })).toBeInTheDocument();
    expect(row).not.toContainElement(choice());
    expect(row.nextElementSibling).toContainElement(choice());
  });

  // Unfold eases a line opening and closing, not a swap: Ending soon's pills and
  // A group's fields must be one height, or swapping lenses jumps the page below.
  it("draws A group's fields dense, the height of the pills they replace", () => {
    query = "lens=group&group=team%3Alasers";
    render(<Mount />);
    expect(screen.getByRole("combobox", { name: "Which group" }).className).toContain(DENSE_FIELD);
    expect(screen.getByRole("textbox", { name: "Team name" }).className).toContain(DENSE_FIELD);
  });
});

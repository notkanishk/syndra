// @vitest-environment jsdom
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AnswerPanel, type Panel } from "@/components/insights/AnswerPanel";
import { buildModel, lightOf } from "@/components/insights/model";

let query = "";
vi.mock("next/navigation", () => ({
  usePathname: () => "/insights",
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(query),
}));

const graph = {
  nodes: [
    { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    { id: "role:p1:op", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "op" } },
    { id: "project:p2", label: "Wood Shop", kind: "project", project_id: "p2" },
    { id: "role:p2:sa", label: "Saw", kind: "role", project_id: "p2", meta: { role_key: "sa" } },
  ],
  edges: [
    { id: "e1", source: "project:p1", target: "role:p1:op", kind: "contains", label: "" },
    { id: "e2", source: "project:p2", target: "role:p2:sa", kind: "contains", label: "" },
  ],
};
vi.mock("@/lib/queries/useTopology", () => ({ useTopology: () => ({ data: graph, dataUpdatedAt: 0, isLoading: false, isError: false }) }));
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

const panel = (onLine = true): Panel => ({
  tone: "accent",
  eyebrow: "What if",
  title: "Lab bundle",
  lines: [
    { tone: "warn", lead: "Priya", text: "would lose Operator.", lights: onLine ? { nodes: ["role:p1:op"], threads: ["e1"] } : undefined },
    { tone: "ink", text: "Nothing else changes." },
  ],
});

describe("lightOf: what a named person or role lights", () => {
  const model = buildModel(graph as never);

  it("paints the named nodes and threads in the line's tone, and nothing else", () => {
    const h = lightOf(model, { nodes: ["role:p1:op"], threads: ["e1"] }, "warn");
    expect(h?.nodes.get("role:p1:op")).toBe("warn");
    expect(h?.threads.get("e1")).toBe("warn");
    expect(h?.nodes.has("role:p2:sa")).toBe(false);
  });

  it("drops what is not on the map, and lights nothing when none of it is", () => {
    expect(lightOf(model, { nodes: ["role:gone"] }, "accent")).toBeNull();
    expect(lightOf(model, { nodes: ["role:gone", "role:p2:sa"] }, "accent")?.nodes.size).toBe(1);
  });
});

describe("a line in the answer lights its thread (A-06 section 7)", () => {
  it("hover and focus say what to light, and leaving or blurring says nothing", () => {
    const onLight = vi.fn();
    render(<AnswerPanel panel={panel()} onLight={onLight} />);
    const line = screen.getByText("would lose Operator.").closest("p")!;
    expect(line).toHaveAttribute("tabindex", "0");

    fireEvent.mouseEnter(line);
    expect(onLight).toHaveBeenLastCalledWith({ nodes: ["role:p1:op"], threads: ["e1"], tone: "warn" });
    fireEvent.mouseLeave(line);
    expect(onLight).toHaveBeenLastCalledWith(null);

    fireEvent.focus(line);
    expect(onLight).toHaveBeenLastCalledWith({ nodes: ["role:p1:op"], threads: ["e1"], tone: "warn" });
    fireEvent.blur(line);
    expect(onLight).toHaveBeenLastCalledWith(null);
  });

  it("a line that names nothing on the map is not a stop", () => {
    render(<AnswerPanel panel={panel()} onLight={vi.fn()} />);
    expect(screen.getByText("Nothing else changes.").closest("p")).not.toHaveAttribute("tabindex");
  });

  it("with no map to light (below tablet) the lines are plain text", () => {
    render(<AnswerPanel panel={panel()} />);
    expect(screen.getByText("would lose Operator.").closest("p")).not.toHaveAttribute("tabindex");
  });

  it("lets go of the map when the answer goes", () => {
    const onLight = vi.fn();
    const { unmount } = render(<AnswerPanel panel={panel()} onLight={onLight} />);
    unmount();
    expect(onLight).toHaveBeenLastCalledWith(null);
  });
});

function Mount() {
  return <MapTab url={useInsightsUrl()} />;
}
const opacity = (node: string) => (document.querySelector(`[data-node="${node}"]`) as HTMLElement).style.opacity;

describe("on the map", () => {
  beforeEach(() => {
    // Two pins: both threads are lit, so neither node is dimmed.
    query = "pin=role:p1:op,role:p2:sa";
  });

  it("hovering one pinned line leaves only its own thread lit, and leaving restores both", () => {
    render(<Mount />);
    expect(opacity("role:p1:op")).toBe("1");
    expect(opacity("role:p2:sa")).toBe("1");

    const section = screen.getByRole("region", { name: /./ });
    const line = within(section).getByText(/Operator: /).closest("p")!;
    act(() => {
      fireEvent.mouseEnter(line);
    });
    expect(opacity("role:p1:op")).toBe("1");
    expect(opacity("role:p2:sa")).toBe("0.35");
    expect(opacity("project:p2")).toBe("0.35");

    act(() => {
      fireEvent.mouseLeave(line);
    });
    expect(opacity("role:p2:sa")).toBe("1");
  });
});

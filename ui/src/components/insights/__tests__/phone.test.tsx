// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setMediaQuery } from "@/test-utils/media";

const PHONE = "(max-width: 44.99rem)";
const TOUCH = "(max-width: 67.49rem)";

let query = "";
const replace = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => "/insights",
  useRouter: () => ({ replace }),
  useSearchParams: () => new URLSearchParams(query),
}));

const graph = {
  nodes: [
    { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
    { id: "role:p1:op", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "op" } },
  ],
  edges: [{ id: "e1", source: "project:p1", target: "role:p1:op", kind: "contains", label: "" }],
};
vi.mock("@/lib/queries/useTopology", () => ({ useTopology: () => ({ data: graph, dataUpdatedAt: 0, isLoading: false, isError: false }) }));
vi.mock("@/lib/queries/useCatalogUsers", () => ({ useCatalogUsers: () => ({ data: [] }) }));

const asOf = { data: undefined as unknown, isError: false };
vi.mock("@/lib/queries/useInsights", () => ({
  useInsightsAsOf: () => asOf,
  useInsightsLens: () => ({ data: undefined }),
  useInsightsTimeline: () => ({ data: undefined }),
  useInsightsUsage: () => ({ data: undefined }),
}));
vi.mock("@/components/insights/useAnswer", () => ({ useAnswer: () => ({ answer: null, status: null, pin: null }) }));

const rest = [
  {
    finding: "drift",
    kind: "Drift",
    title: "Operator is held in Zitadel, not given by Syndra",
    line: "2 people. They keep it until you decide.",
    open: { label: "Open on Drift", href: "/governance/drift" },
  },
  {
    finding: "send",
    kind: "Waiting for you to send",
    title: "1 change is approved, not yet in Zitadel",
    line: "Operator for Aanya.",
    open: { label: "Waiting for you to send", href: "/governance/pending" },
  },
];
vi.mock("@/components/insights/useFindings", () => ({
  useFindings: () => ({
    ready: true,
    rest,
    counts: { drift: 1, send: 1, single: 0, unused: 0, overlap: 0 },
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

beforeEach(() => {
  replace.mockReset();
  query = "";
  asOf.data = undefined;
  setMediaQuery(PHONE, true);
  setMediaQuery(TOUCH, true);
});

describe("Insights below tablet (20a)", () => {
  it("at rest lists what is worth a look, each with its link, and draws no map, list switch or finding chips", () => {
    render(<Mount />);
    expect(screen.getByText("Worth a look")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open on Drift/ })).toHaveAttribute("href", "/governance/drift");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
    expect(screen.queryByRole("radiogroup", { name: "View" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Earlier date" })).not.toBeInTheDocument();
    // Structure that is not the answer: no map, no list of nodes.
    expect(screen.queryByText("Operator", { selector: "button *" })).not.toBeInTheDocument();
  });

  it("turns the lens into a Select, and choosing one writes it to the link", () => {
    render(<Mount />);
    expect(screen.queryByRole("radiogroup", { name: "Lens" })).not.toBeInTheDocument();
    const lens = screen.getByRole("combobox", { name: "Lens" });
    expect(screen.getByRole("option", { name: "Lens: People" })).toBeInTheDocument();
    fireEvent.change(lens, { target: { value: "ending" } });
    expect(replace).toHaveBeenCalledWith(expect.stringContaining("lens=ending"), { scroll: false });
  });

  it("with a question, the panel is the page: its sentences, Pin this and Copy as sentences, and where the map is", () => {
    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn() }, configurable: true });
    query = "q=role:p1:op";
    render(<Mount />);
    expect(screen.queryByText("Worth a look")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Laser Lab · Operator" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Pin this" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy as sentences" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
    expect(screen.getByText(/The map draws this on a wider screen/)).toBeInTheDocument();
  });

  it("does not show today's findings under an earlier date while that day is still loading", () => {
    query = "asof=2026-10-01";
    render(<Mount />);
    expect(screen.getByText(/Showing/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back to now" })).toBeInTheDocument();
    expect(screen.queryByText("Worth a look")).not.toBeInTheDocument();
  });

  it("keeps the wider screen as it was: chips and the View switch come back", () => {
    setMediaQuery(PHONE, false);
    setMediaQuery(TOUCH, false);
    render(<Mount />);
    expect(screen.queryByText("Worth a look")).not.toBeInTheDocument();
    expect(screen.getByRole("radiogroup", { name: "Lens" })).toBeInTheDocument();
    expect(screen.getByRole("radiogroup", { name: "View" })).toBeInTheDocument();
  });
});

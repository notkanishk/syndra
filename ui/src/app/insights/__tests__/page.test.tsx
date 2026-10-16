// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import InsightsPage from "@/app/insights/page";
import type { TopologyGraph } from "@/lib/queries/useTopology";

const topology = vi.hoisted(() => ({
  data: undefined as TopologyGraph | undefined,
  isLoading: false,
  isError: false,
  error: null as unknown,
  dataUpdatedAt: Date.now(),
}));
const reads = vi.hoisted(() => ({
  drift: [] as unknown[],
  pending: [] as unknown[],
  catalog: [] as unknown[],
}));
const viewport = vi.hoisted(() => ({ phone: false, touch: false }));
const insights = vi.hoisted(() => ({
  findings: { data: undefined as unknown, isError: false },
  who: { data: undefined as unknown, isError: false },
  whatIf: { data: undefined as unknown, isError: false },
  asof: { data: undefined as unknown, isError: false },
  trends: { data: undefined as unknown, isLoading: false, error: null as unknown, refetch: () => {} },
  lens: { data: undefined as unknown, isError: false },
  usage: { data: undefined as unknown, isError: false },
  calls: {} as { who?: [string | null, string | null]; change?: unknown; lens?: unknown },
}));
const people = vi.hoisted(() => ({
  users: [] as Array<{ id: string; name: string; title: string; email: string }>,
  access: {} as Record<string, unknown>,
}));

// The link is the state, so the router here really changes it, and anything
// reading the search string re-renders.
const nav = vi.hoisted(() => {
  const listeners = new Set<() => void>();
  const state = { search: "" };
  return {
    state,
    listeners,
    set(search: string) {
      state.search = search;
      listeners.forEach((l) => l());
    },
  };
});

vi.mock("next/navigation", async () => {
  const React = await import("react");
  return {
    usePathname: () => "/insights",
    useRouter: () => ({ replace: (url: string) => nav.set(url.split("?")[1] ?? "") }),
    useSearchParams: () => {
      const search = React.useSyncExternalStore(
        (cb) => {
          nav.listeners.add(cb);
          return () => void nav.listeners.delete(cb);
        },
        () => nav.state.search,
        () => nav.state.search,
      );
      return new URLSearchParams(search);
    },
  };
});

vi.mock("@/lib/queries/useTopology", () => ({
  useTopology: () => ({ ...topology, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useDrift", () => ({
  useDriftItems: () => ({ data: reads.drift }),
}));
vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: () => ({ data: reads.pending }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: reads.catalog }),
}));
vi.mock("@/lib/queries/useInsights", () => ({
  useInsightsFindings: () => insights.findings,
  useInsightsWho: (has: string | null, without: string | null) => {
    insights.calls.who = [has, without];
    return has ? insights.who : { data: undefined, isError: false };
  },
  useInsightsWhatIf: (change: unknown) => {
    insights.calls.change = change;
    return change ? insights.whatIf : { data: undefined, isError: false };
  },
  useInsightsTrends: () => insights.trends,
  useInsightsAsOf: () => insights.asof,
  useInsightsLens: (ask: unknown) => {
    insights.calls.lens = ask;
    return ask ? insights.lens : { data: undefined, isError: false };
  },
  useInsightsUsage: () => insights.usage,
  useInsightsTimeline: () => ({ data: undefined }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogUsers: () => ({ data: people.users }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: (id: string) => ({ data: people.access[id], isError: false }),
}));
vi.mock("@/lib/useViewport", async (orig) => ({
  ...(await orig<typeof import("@/lib/useViewport")>()),
  useIsPhone: () => viewport.phone,
  useIsTouch: () => viewport.touch,
}));

const drift = (id: string, user: string) => ({
  id,
  target: "zitadel",
  user_id: user,
  project_id: "p1",
  role_keys: ["trained"],
  drift_type: "target_only",
  detection_source: "sweep",
  detected_at: "2026-09-30T10:00:00Z",
});
const role = (project_id: string, role_key: string, assigned: number) => ({
  project_id,
  project_name: "",
  role_key,
  display_name: role_key,
  assigned_user_count: assigned,
  observed_user_count: assigned,
});

const noFindings = { single_routes: [], unused: { roles: [], bundles: [], projects: [] }, overlaps: [] };

beforeEach(() => {
  nav.set("");
  viewport.phone = false;
  viewport.touch = false;
  topology.isLoading = false;
  topology.isError = false;
  topology.error = null;
  topology.data = {
    nodes: [
      { id: "p1", label: "Laser Lab", kind: "project" },
      { id: "p2", label: "Metal Shop", kind: "project" },
      { id: "r1", label: "trained", kind: "role", project_id: "p1", meta: { role_key: "trained" } },
      { id: "r2", label: "welder", kind: "role", project_id: "p2", meta: { role_key: "welder" } },
      { id: "b1", label: "Lab Tech", kind: "bundle" },
      { id: "a1", label: "Badge reader", kind: "application" },
    ],
    edges: [
      { id: "e1", source: "p1", target: "r1", kind: "contains", label: "" },
      { id: "e2", source: "p2", target: "r2", kind: "contains", label: "" },
      { id: "e3", source: "b1", target: "r1", kind: "bundle", label: "" },
      { id: "e4", source: "a1", target: "p1", kind: "application", label: "" },
    ],
  };
  reads.drift = [drift("d1", "u1"), drift("d2", "u2")];
  reads.pending = [
    { id: "o1", op_type: "grant", user_id: "u3", project_id: "p1", role_keys: ["trained"], wait_reason: "role_waits" },
    { id: "o2", op_type: "grant", user_id: "u4", project_id: "p1", role_keys: ["trained"] },
  ];
  reads.catalog = [role("p1", "trained", 5), role("p2", "welder", 0)];
  insights.findings = { data: { ...noFindings, unused: { ...noFindings.unused, roles: [{ project_id: "p2", role_key: "welder" }] } }, isError: false };
  insights.who = { data: undefined, isError: false };
  insights.whatIf = { data: undefined, isError: false };
  insights.asof = { data: undefined, isError: false };
  insights.lens = { data: undefined, isError: false };
  insights.usage = { data: undefined, isError: false };
  insights.calls = {};
  people.users = [];
  people.access = {};
});

const node = (name: string | RegExp) => screen.getByRole("button", { name });
const opacityOf = (name: string | RegExp) => node(name).style.opacity;

describe("tabs and the link", () => {
  it("opens on Map, and the link holds the tab", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "Insights" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Map" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByPlaceholderText("Ask about a person, a door, a role, or find anything")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Trends" }));
    expect(nav.state.search).toBe("tab=trends");
    expect(screen.getByRole("radiogroup", { name: "Period" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Map" }));
    expect(nav.state.search).toBe("");
    expect(screen.getByRole("group", { name: "Access map" })).toBeInTheDocument();
  });

  it("opens straight on Trends from the link", () => {
    nav.set("tab=trends");
    renderPage();
    expect(screen.getByRole("button", { name: "Trends" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("radiogroup", { name: "Period" })).toBeInTheDocument();
  });

  it("switches Map and List in the link, and only offers the switch on a desktop", () => {
    const { unmount } = renderPage();
    fireEvent.click(screen.getByRole("radio", { name: "List" }));
    expect(nav.state.search).toBe("view=list");
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();
    expect(screen.getByText("Given through")).toBeInTheDocument();
    unmount();

    // Below desktop Map and List collapse to List: no switch, no map.
    viewport.touch = true;
    nav.set("");
    renderPage();
    expect(screen.queryByRole("radio", { name: "Map" })).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();
    expect(screen.getByText("Given through")).toBeInTheDocument();
  });

  it("says nothing is drawable rather than showing a blank map", () => {
    topology.data = { nodes: [], edges: [] };
    renderPage();
    expect(screen.getByText("Nothing to show yet.")).toBeInTheDocument();
  });

  it("carries no freshness or counts before anything has been read", () => {
    topology.data = undefined;
    topology.isLoading = true;
    reads.drift = undefined as unknown as unknown[];
    reads.pending = undefined as unknown as unknown[];
    reads.catalog = undefined as unknown as unknown[];
    insights.findings = { data: undefined, isError: false };
    renderPage();
    expect(screen.queryByText(/was read/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /drift/ })).not.toBeInTheDocument();
  });
});

describe("an earlier date", () => {
  it("hides the finding chips, says there is no picture, and the way back to today clears the link", () => {
    nav.set("asof=2020-02-29");
    insights.asof = { data: { kind: "none", first_day: "2026-10-16" }, isError: false };
    renderPage();
    expect(screen.getByText("Showing 29 Feb")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "2 drift" })).not.toBeInTheDocument();
    expect(screen.getByText("Syndra keeps one picture a day, starting 16 Oct 2026. There's none for that date.")).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Date shown" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Back to now" }));
    expect(nav.state.search).toBe("");
    expect(screen.getByRole("button", { name: "2 drift" })).toBeInTheDocument();
  });

  it("opens the date slider from the map without leaving today", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Earlier date" }));
    expect(screen.getByRole("slider", { name: "Date shown" })).toBeInTheDocument();
    expect(nav.state.search).toBe("");
  });
});

describe("the finding chips", () => {
  it("count what the existing reads say, and open their panels", () => {
    renderPage();
    expect(screen.getByRole("button", { name: "2 drift" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "1 waiting for you to send" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "1 unused" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "2 drift" }));
    expect(nav.state.search).toBe("finding=drift");
    expect(screen.getByRole("heading", { name: "Held in Zitadel, not given by Syndra" })).toBeInTheDocument();
    expect(screen.getByText("Laser Lab · trained: 2 people.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open on Drift/ })).toHaveAttribute("href", "/governance/drift");

    fireEvent.click(screen.getByRole("button", { name: "1 waiting for you to send" }));
    expect(screen.getByText("Approved, not yet in Zitadel")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "1 change" })).toBeInTheDocument();
    expect(screen.getByText("Gives Laser Lab · trained to 1 person once sent.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");

    fireEvent.click(screen.getByRole("button", { name: "1 unused" }));
    expect(screen.getByRole("heading", { name: "1 thing gives nothing" })).toBeInTheDocument();
    expect(screen.getByText("in Metal Shop that nobody has.")).toBeInTheDocument();
  });

  it("only counts rows that wait, and hides a chip with nothing behind it", () => {
    reads.drift = [];
    reads.catalog = [role("p1", "trained", 5), role("p2", "welder", 3)];
    insights.findings = { data: noFindings, isError: false };
    renderPage();
    expect(screen.queryByRole("button", { name: /drift/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /unused/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "1 waiting for you to send" })).toBeInTheDocument();
  });

  it("toggles the panel off again, and Esc steps back", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "2 drift" }));
    fireEvent.click(screen.getByRole("button", { name: "2 drift" }));
    expect(nav.state.search).toBe("");

    fireEvent.click(screen.getByRole("button", { name: "2 drift" }));
    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(nav.state.search).toBe("");
    expect(screen.queryByRole("heading", { name: "Held in Zitadel, not given by Syndra" })).not.toBeInTheDocument();
  });
});

describe("following a node", () => {
  it("dims everything the answer does not reach, and says it in a sentence", () => {
    renderPage();
    fireEvent.click(node(/Laser Lab · trained/));
    expect(nav.state.search).toBe("q=r1");

    expect(
      screen.getByText("Laser Lab · trained is given by Lab Tech and read by Badge reader."),
    ).toBeInTheDocument();
    expect(screen.getByText("Role · Laser Lab · 5 people")).toBeInTheDocument();

    // What gives it and what it gives stay; the rest steps back to 0.35.
    expect(opacityOf("Lab Tech")).toBe("1");
    expect(opacityOf("Laser Lab")).toBe("1");
    expect(opacityOf("Badge reader")).toBe("1");
    expect(opacityOf("Metal Shop")).toBe("0.35");
    expect(opacityOf(/Metal Shop · welder/)).toBe("0.35");
  });

  it("finds a node by name in the question and opens it", () => {
    renderPage();
    const ask = screen.getByLabelText("Ask the map");
    fireEvent.change(ask, { target: { value: "badge" } });
    fireEvent.keyDown(ask, { key: "Enter" });
    expect(nav.state.search).toBe("q=a1");
    expect(screen.getByText("Badge reader reads Laser Lab.")).toBeInTheDocument();
    expect(screen.getByText("Access reaches it through Lab Tech.")).toBeInTheDocument();
  });

  it("moves along a lane with the arrow keys, and opens the keys with ?", () => {
    nav.set("q=p1");
    renderPage();
    fireEvent.keyDown(document.body, { key: "ArrowDown" });
    expect(nav.state.search).toBe("q=p2");

    fireEvent.keyDown(document.body, { key: "?" });
    expect(within(screen.getByRole("dialog", { name: "Keys" })).getByText("Back one question")).toBeInTheDocument();
    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Keys" })).not.toBeInTheDocument();
    expect(nav.state.search).toBe("q=p2");
  });
});

describe("pins", () => {
  it("keeps up to three, and offers no fourth", () => {
    renderPage();
    for (const [name, title] of [
      [/Laser Lab · trained/, "One thread"],
      ["Lab Tech", "Two threads, side by side"],
      ["Badge reader", "Three threads, side by side"],
    ] as const) {
      fireEvent.click(node(name));
      fireEvent.click(screen.getByRole("button", { name: "Pin" }));
      expect(title).toBeTruthy();
    }
    expect(nav.state.search.split("&").find((p) => p.startsWith("pin="))!.split("%2C")).toHaveLength(3);

    // A fourth thing followed cannot be pinned.
    fireEvent.click(node("Metal Shop"));
    expect(screen.queryByRole("button", { name: "Pin" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unpin Lab Tech" })).toBeInTheDocument();

    // With nothing followed, the pins are the question.
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByRole("heading", { name: "Three threads, side by side" })).toBeInTheDocument();
    expect(screen.getByText("Where they cross: Badge reader reads more than one of them.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Unpin Lab Tech" }));
    expect(screen.getByRole("heading", { name: "Two threads, side by side" })).toBeInTheDocument();
  });

  it("pins what is followed with P", () => {
    nav.set("q=b1");
    renderPage();
    fireEvent.keyDown(document.body, { key: "p" });
    expect(nav.state.search).toContain("pin=b1");
    fireEvent.keyDown(document.body, { key: "p" });
    expect(nav.state.search).not.toContain("pin=");
  });
});

describe("Copy as sentences", () => {
  it("writes the panel's sentences to the clipboard, then says Copied for a moment", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    nav.set("q=r1");
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Copy as sentences" }));
    expect(writeText).toHaveBeenCalledWith(
      "Laser Lab · trained\nLaser Lab · trained is given by Lab Tech and read by Badge reader.",
    );
    expect(await screen.findByRole("button", { name: "Copied" })).toBeInTheDocument();
  });
});

describe("below tablet", () => {
  it("lists what is worth a look, each with its link, and the lens is a Select", () => {
    viewport.phone = true;
    viewport.touch = true;
    renderPage();

    expect(screen.getByRole("combobox", { name: "Lens" })).toHaveValue("people");
    expect(screen.getByText("Worth a look")).toBeInTheDocument();
    expect(screen.getByText("Laser Lab · trained is held in Zitadel, not given by Syndra")).toBeInTheDocument();
    expect(screen.getByText("1 change is approved, not yet in Zitadel")).toBeInTheDocument();
    expect(screen.getByText("1 thing gives nothing")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open on Drift/ })).toHaveAttribute("href", "/governance/drift");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
    expect(screen.getByRole("link", { name: /Open Metal Shop/ })).toHaveAttribute("href", "/projects/p2");

    // No chips, no switch and no map: the answer is the page.
    expect(screen.queryByRole("button", { name: "2 drift" })).not.toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Map" })).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();
  });

  it("answers a question in full, in place of the cards", () => {
    viewport.phone = true;
    viewport.touch = true;
    nav.set("q=r1");
    renderPage();

    expect(screen.queryByText("Worth a look")).not.toBeInTheDocument();
    expect(
      screen.getByText("Laser Lab · trained is given by Lab Tech and read by Badge reader."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open Laser Lab/ })).toHaveAttribute("href", "/projects/p1");
    expect(screen.getByRole("button", { name: "Pin this" })).toBeInTheDocument();
    expect(screen.getByText(/The map draws this on a wider screen/)).toBeInTheDocument();
  });
});

// A richer map, in the ids the API really uses, for the findings and the questions.
const BUNDLE = (n: number) => `bundle:b${n}`;
function richMap() {
  topology.data = {
    nodes: [
      { id: "project:p1", label: "Laser Lab", kind: "project", project_id: "p1" },
      { id: "project:p2", label: "Textiles", kind: "project", project_id: "p2" },
      { id: "role:p1:operator", label: "Operator", kind: "role", project_id: "p1", meta: { role_key: "operator" } },
      { id: "role:p1:door", label: "Door", kind: "role", project_id: "p1", meta: { role_key: "door" } },
      { id: "role:p2:weaving", label: "Weaving", kind: "role", project_id: "p2", meta: { role_key: "weaving" } },
      { id: BUNDLE(1), label: "Lab Tech", kind: "bundle" },
      { id: BUNDLE(2), label: "New members", kind: "bundle" },
      { id: BUNDLE(3), label: "Safety basics", kind: "bundle" },
    ],
    edges: [
      { id: "contains:p1:operator", source: "project:p1", target: "role:p1:operator", kind: "contains", label: "" },
      { id: "contains:p1:door", source: "project:p1", target: "role:p1:door", kind: "contains", label: "" },
      { id: "contains:p2:weaving", source: "project:p2", target: "role:p2:weaving", kind: "contains", label: "" },
      { id: "bundle-role:b1:p1:operator", source: BUNDLE(1), target: "role:p1:operator", kind: "bundle", label: "" },
      { id: "bundle-role:b1:p1:door", source: BUNDLE(1), target: "role:p1:door", kind: "bundle", label: "" },
      { id: "bundle-role:b2:p1:operator", source: BUNDLE(2), target: "role:p1:operator", kind: "bundle", label: "" },
      { id: "rule:x1", source: "role:p1:operator", target: "role:p1:door", kind: "rule", label: "" },
    ],
  };
  reads.drift = [];
  reads.pending = [];
  reads.catalog = [role("p1", "operator", 51), role("p1", "door", 22), role("p2", "weaving", 0)];
  people.users = [
    { id: "u1", name: "Priya Nair", title: "Research assistant", email: "priya@example.edu" },
    { id: "u2", name: "Aanya Rao", title: "Student", email: "aanya@example.edu" },
  ];
}
const panelOf = (name: string | RegExp) => screen.getByRole("region", { name });
const view = (id: string, name: string, door: unknown[]) => ({
  user: { id, name, email: "", title: "", team: "", status: "active", avatar: "" },
  projects: [
    {
      project_id: "p1",
      project_name: "Laser Lab",
      project_name_resolved: true,
      source_roles: [],
      derived_roles: door.length ? [{ role_key: "door", reasons: door }] : [],
      effective_role_keys: [],
      observed_role_keys: null,
    },
  ],
});
const hand = { kind: "direct", description: "Direct Zitadel grant" };
const labTech = { kind: "bundle", description: "", bundle_id: "b1", bundle_name: "Lab Tech" };
const byRule = { kind: "mapping", description: "", trigger_project: "p1", trigger_role: "operator" };

describe("the findings read from how access is arranged", () => {
  beforeEach(richMap);

  it("a single route says what depends on what, and marks the bundle on the map", () => {
    insights.findings = {
      data: {
        ...noFindings,
        single_routes: [{ project_id: "p1", role_key: "door", source: { kind: "bundle", id: "b1", name: "Lab Tech" }, only_that_way: 48, holders: 53 }],
      },
      isError: false,
    };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "1 single route" }));
    expect(nav.state.search).toBe("finding=single");

    const panel = panelOf("Laser Lab · Door depends on Lab Tech");
    expect(panel).toHaveTextContent("48 of 53 people get it only that way");
    expect(panel).toHaveTextContent("A change to Lab Tech reaches Laser Lab · Door for 48 people.");
    expect(panel).toHaveTextContent("The other 5 get it another way.");
    expect(within(panel).getByRole("link", { name: /Open Lab Tech/ })).toHaveAttribute("href", "/bundles/b1");
    expect(screen.getByTitle("Single route")).toBeInTheDocument();
    // The role and its bundle stay lit; what the finding does not reach steps back.
    expect(opacityOf("Lab Tech")).toBe("1");
    expect(opacityOf(/Laser Lab · Door/)).toBe("1");
    expect(opacityOf("New members")).toBe("0.35");
  });

  it("unused names the projects nobody reaches, bundles that add nothing, and roles nobody has", () => {
    insights.findings = {
      data: {
        ...noFindings,
        unused: {
          roles: [
            { project_id: "p2", role_key: "weaving" },
            { project_id: "p1", role_key: "spare" },
          ],
          bundles: [{ bundle_id: "b3", name: "Safety basics", holders: 9 }],
          projects: [{ project_id: "p2", name: "Textiles" }],
        },
      },
      isError: false,
    };
    renderPage();
    // Textiles' own role is counted inside Textiles, not twice.
    fireEvent.click(screen.getByRole("button", { name: "3 unused" }));
    const panel = panelOf("3 things give nothing");
    expect(panel).toHaveTextContent("Textiles: 1 role, and nobody has it.");
    expect(panel).toHaveTextContent("Safety basics: its 9 people already have everything it gives.");
    expect(panel).toHaveTextContent("1 role in Laser Lab that nobody has.");
    expect(within(panel).getByRole("link", { name: /Open Textiles/ })).toHaveAttribute("href", "/projects/p2");
  });

  it("an overlap suggests including the smaller bundle in the larger", () => {
    insights.findings = {
      data: {
        ...noFindings,
        overlaps: [
          {
            a: { bundle_id: "b1", name: "Lab Tech" },
            b: { bundle_id: "b2", name: "New members" },
            shared: [{ project_id: "p1", role_key: "operator" }],
            a_only: [{ project_id: "p1", role_key: "door" }],
            b_only: [],
          },
        ],
      },
      isError: false,
    };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "1 overlap" }));
    const panel = panelOf("Lab Tech and New members");
    expect(panel).toHaveTextContent("1 role in common · Lab Tech adds 1");
    expect(panel).toHaveTextContent("Both give Laser Lab · Operator.");
    expect(panel).toHaveTextContent("Only Lab Tech gives Laser Lab · Door.");
    expect(panel).toHaveTextContent("You could include New members inside Lab Tech, so a change to the shared role is made once.");
    expect(within(panel).getByRole("link", { name: /Open Lab Tech/ })).toHaveAttribute("href", "/bundles/b1");
  });

  it("an overlap where each gives something the other doesn't does not suggest including", () => {
    insights.findings = {
      data: {
        ...noFindings,
        overlaps: [
          {
            a: { bundle_id: "b1", name: "Lab Tech" },
            b: { bundle_id: "b2", name: "New members" },
            shared: [{ project_id: "p1", role_key: "operator" }],
            a_only: [{ project_id: "p1", role_key: "door" }],
            b_only: [{ project_id: "p2", role_key: "weaving" }],
          },
        ],
      },
      isError: false,
    };
    nav.set("finding=overlap");
    renderPage();
    const panel = panelOf("Lab Tech and New members");
    expect(panel).not.toHaveTextContent("You could include");
    expect(panel).toHaveTextContent("would change who gets what");
  });
});

describe("at rest", () => {
  beforeEach(richMap);

  it("says three things worth knowing, each from what was read", () => {
    insights.findings = {
      data: { ...noFindings, unused: { roles: [], bundles: [], projects: [{ project_id: "p2", name: "Textiles" }] } },
      isError: false,
    };
    renderPage();
    const panel = panelOf("How access travels today");
    expect(panel).toHaveTextContent("The makerspace at a glance");
    expect(panel).toHaveTextContent("Three things worth knowing");
    expect(panel).toHaveTextContent("Widest reach: Laser Lab · Operator, 51 people, through Lab Tech and New members.");
    expect(panel).toHaveTextContent("Nothing reaches: Textiles. It has 1 role and nobody has it.");
    expect(panel).toHaveTextContent("Most given automatically: Laser Lab · Door, 51 people, by rule from Laser Lab · Operator.");
    expect(within(panel).queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
  });

  it("drops a line it cannot compute rather than guess it", () => {
    reads.catalog = [];
    renderPage();
    expect(screen.queryByRole("region", { name: "How access travels today" })).not.toBeInTheDocument();
  });
});

describe("the question seat", () => {
  beforeEach(richMap);
  const type = (value: string) => fireEvent.change(screen.getByLabelText("Ask the map"), { target: { value } });
  const option = (name: RegExp) => screen.getByRole("option", { name });

  it("offers the questions its names can ask, and keeps the chosen one in the link", () => {
    renderPage();
    type("priya to door");
    expect(option(/Path from Priya Nair to Laser Lab · Door/)).toBeInTheDocument();
    fireEvent.click(option(/Path from Priya Nair to Laser Lab · Door/));

    expect(new URLSearchParams(nav.state.search).get("q")).toBe("path");
    expect(new URLSearchParams(nav.state.search).get("person")).toBe("u1");
    expect(new URLSearchParams(nav.state.search).get("role")).toBe("role:p1:door");
    const chips = screen.getByRole("group", { name: "The question the map is answering" });
    expect(chips).toHaveTextContent("Path from Priya Nair to Laser Lab · Door");
    expect(screen.queryByLabelText("Ask the map")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Clear the question" }));
    expect(nav.state.search).toBe("");
    expect(screen.getByLabelText("Ask the map")).toBeInTheDocument();
  });

  it("opens a question straight from a link, with the pins kept", () => {
    people.access = { u1: view("u1", "Priya Nair", [hand]) };
    nav.set("q=path&person=u1&role=role%3Ap1%3Adoor&pin=bundle%3Ab1");
    renderPage();
    expect(screen.getByRole("group", { name: "The question the map is answering" })).toHaveTextContent("Path from Priya Nair to Laser Lab · Door");
    expect(screen.getByRole("button", { name: "Unpin Lab Tech" })).toBeInTheDocument();
  });

  it("ignores a link naming something no longer on the map", () => {
    nav.set("q=gone&bundle=bundle%3Aold");
    renderPage();
    expect(screen.queryByRole("group", { name: "The question the map is answering" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Ask the map")).toBeInTheDocument();
  });

  it("moves through the offers with the arrow keys and asks the highlighted one", () => {
    renderPage();
    type("lab");
    const ask = screen.getByLabelText("Ask the map");
    fireEvent.keyDown(ask, { key: "ArrowDown" });
    fireEvent.keyDown(ask, { key: "Enter" });
    expect(nav.state.search).toContain("q=");
    expect(screen.getByRole("group", { name: "The question the map is answering" })).toBeInTheDocument();
  });
});

describe("the answers", () => {
  beforeEach(richMap);

  it("Path says each route in the order by hand, bundle, rule, and the map lights them", () => {
    people.access = { u1: view("u1", "Priya Nair", [hand, labTech, byRule]) };
    nav.set("q=path&person=u1&role=role%3Ap1%3Adoor");
    renderPage();

    const panel = panelOf("Priya Nair to Laser Lab · Door");
    expect(panel).toHaveTextContent("Path · 3 routes");
    expect(panel).toHaveTextContent("Research assistant");
    expect(panel).toHaveTextContent("By hand: given to them directly.");
    expect(panel).toHaveTextContent("Through Lab Tech: Lab Tech gives Laser Lab · Door.");
    expect(panel).toHaveTextContent("By rule from Laser Lab · Operator: Laser Lab · Door comes automatically with Laser Lab · Operator.");
    expect(panel).toHaveTextContent("3 routes. Removing one of them won't stop Priya Nair having it.");
    expect(within(panel).getByRole("link", { name: /Open Priya Nair/ })).toHaveAttribute("href", "/users/u1");
    expect(opacityOf("Lab Tech")).toBe("1");
    expect(opacityOf("New members")).toBe("0.35");

    // Pin this pins the thread the question is about.
    fireEvent.click(within(panel).getByRole("button", { name: "Pin" }));
    expect(nav.state.search).toContain("pin=role%3Ap1%3Adoor");
  });

  it("Path says so when the person doesn't have it, and names a bundle's child", () => {
    people.access = { u1: view("u1", "Priya Nair", []) };
    nav.set("q=path&person=u1&role=role%3Ap1%3Adoor");
    const { unmount } = renderPage();
    expect(panelOf("Priya Nair to Laser Lab · Door")).toHaveTextContent("Priya Nair doesn't have Laser Lab · Door.");
    unmount();

    people.access = {
      u1: view("u1", "Priya Nair", [{ ...labTech, bundle_name: "New members", via_bundle_id: "b3", via_bundle_name: "Safety basics", queued: true }]),
    };
    renderPage();
    const panel = panelOf("Priya Nair to Laser Lab · Door");
    expect(panel).toHaveTextContent("Through New members, from Safety basics: New members includes Safety basics");
    expect(panel).toHaveTextContent("It is waiting for you to send.");
  });

  it("Compare sets the shared route apart from each person's own", () => {
    people.access = { u1: view("u1", "Priya Nair", [hand, labTech]), u2: view("u2", "Aanya Rao", [labTech]) };
    nav.set("q=compare&person=u1&other=u2&role=role%3Ap1%3Adoor");
    renderPage();

    const panel = panelOf("Priya Nair and Aanya Rao");
    expect(panel).toHaveTextContent("Compare · on Laser Lab · Door");
    expect(panel).toHaveTextContent("Both have it.");
    expect(panel).toHaveTextContent("Both: through Lab Tech.");
    expect(panel).toHaveTextContent("Only Priya Nair: by hand.");
    expect(panel).not.toHaveTextContent("Only Aanya Rao");
    expect(within(panel).getByRole("link", { name: /Open Priya Nair/ })).toBeInTheDocument();
  });

  it("Compare says who has it when only one does", () => {
    people.access = { u1: view("u1", "Priya Nair", [labTech]), u2: view("u2", "Aanya Rao", []) };
    nav.set("q=compare&person=u1&other=u2&role=role%3Ap1%3Adoor");
    renderPage();
    const panel = panelOf("Priya Nair and Aanya Rao");
    expect(panel).toHaveTextContent("Priya Nair has it, Aanya Rao doesn't.");
    expect(panel).toHaveTextContent("Aanya Rao doesn't have Laser Lab · Door.");
    expect(panel).toHaveTextContent("Only Priya Nair: through Lab Tech.");
  });

  it("Who lists people with how they hold it, and asks with the references the API takes", () => {
    insights.who = {
      data: [
        { user_id: "u7", name: "Rohan Mehta", email: "", how: [hand, labTech] },
        { user_id: "u8", name: "Ishaan Kapoor", email: "", how: [labTech] },
      ],
      isError: false,
    };
    nav.set("q=who&has=role%3Ap1%3Adoor&without=bundle%3Ab3");
    renderPage();

    expect(insights.calls.who).toEqual(["role:p1:door", "bundle:b3"]);
    const panel = panelOf("2 people");
    expect(panel).toHaveTextContent("Who has Laser Lab · Door without Safety basics");
    expect(panel).toHaveTextContent("Rohan Mehta: by hand and through Lab Tech.");
    expect(panel).toHaveTextContent("Ishaan Kapoor: through Lab Tech.");
    expect(within(panel).getByRole("link", { name: /See them in People/ })).toHaveAttribute("href", "/users");
  });

  it("Who with nobody says so, and with one person opens them", () => {
    insights.who = { data: [], isError: false };
    nav.set("q=who&has=role%3Ap1%3Adoor&without=bundle%3Ab3");
    const { unmount } = renderPage();
    expect(panelOf("Nobody")).toHaveTextContent("Everyone who has Laser Lab · Door also has Safety basics.");
    unmount();

    insights.who = { data: [{ user_id: "u7", name: "Rohan Mehta", email: "", how: [hand] }], isError: false };
    renderPage();
    expect(within(panelOf("1 person")).getByRole("link", { name: /Open Rohan Mehta/ })).toHaveAttribute("href", "/users/u7");
  });

  it("What if it went is amber, a preview, and names who would lose what", () => {
    insights.whatIf = {
      data: { gains: [], losses: [{ user_id: "u9", name: "Dev Patel", roles: [{ project_id: "p1", role_key: "door" }] }], unchanged: 11 },
      isError: false,
    };
    nav.set("q=gone&bundle=bundle%3Ab1");
    renderPage();

    expect(insights.calls.change).toEqual({ kind: "bundle_gone", bundle_id: "b1" });
    const panel = panelOf("Lab Tech");
    expect(panel).toHaveTextContent("What if it went · a preview, nothing changes");
    expect(panel).toHaveTextContent("1 person would lose something.");
    expect(panel).toHaveTextContent("Dev Patel would lose Laser Lab · Door.");
    expect(panel).toHaveTextContent("The rest: 11 people would stay as they are.");
    expect(panel.className).toContain("border-warn-line");
    expect(within(panel).getByRole("link", { name: /Change it on Lab Tech/ })).toHaveAttribute("href", "/bundles/b1");
  });

  it("What if it also gave is violet, and names who would gain", () => {
    insights.whatIf = {
      data: { gains: [{ user_id: "u9", name: "Dev Patel", roles: [{ project_id: "p1", role_key: "door" }] }], losses: [], unchanged: 3 },
      isError: false,
    };
    nav.set("q=gives&bundle=bundle%3Ab2&role=role%3Ap1%3Adoor");
    renderPage();

    expect(insights.calls.change).toEqual({ kind: "bundle_gives_also", bundle_id: "b2", project_id: "p1", role_key: "door" });
    const panel = panelOf("New members also gave Laser Lab · Door");
    expect(panel).toHaveTextContent("a preview, nothing changes");
    expect(panel).toHaveTextContent("Dev Patel would gain Laser Lab · Door.");
    expect(panel.className).toContain("border-accent-line");
  });

  it("What if with nothing to move says so", () => {
    insights.whatIf = { data: { gains: [], losses: [], unchanged: 4 }, isError: false };
    nav.set("q=gone&bundle=bundle%3Ab3");
    renderPage();
    expect(panelOf("Safety basics")).toHaveTextContent("Nobody would gain or lose anything.");
  });

  it("says nothing of an answer that has not been read, or could not be", () => {
    nav.set("q=who&has=role%3Ap1%3Adoor");
    insights.who = { data: undefined, isError: false };
    const { unmount } = renderPage();
    expect(panelOf("Who has Laser Lab · Door")).toHaveTextContent("Working it out.");
    expect(screen.queryByText(/people/)).not.toBeInTheDocument();
    unmount();

    insights.who = { data: undefined, isError: true };
    renderPage();
    expect(panelOf("Who has Laser Lab · Door")).toHaveTextContent("Couldn't load this answer.");
  });

  it("Esc goes back one question and keeps the pins", () => {
    people.access = { u1: view("u1", "Priya Nair", [hand]) };
    nav.set("q=path&person=u1&role=role%3Ap1%3Adoor&pin=bundle%3Ab1");
    renderPage();
    expect(screen.getByRole("group", { name: "The question the map is answering" })).toBeInTheDocument();

    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(new URLSearchParams(nav.state.search).get("q")).toBeNull();
    expect(new URLSearchParams(nav.state.search).get("person")).toBeNull();
    expect(new URLSearchParams(nav.state.search).get("role")).toBeNull();
    expect(new URLSearchParams(nav.state.search).get("pin")).toBe("bundle:b1");
    expect(screen.queryByRole("group", { name: "The question the map is answering" })).not.toBeInTheDocument();
  });

  it("on a phone the panel is the page, with the way to the person and the footer line", () => {
    viewport.phone = true;
    viewport.touch = true;
    people.access = { u1: view("u1", "Priya Nair", [hand, labTech]) };
    nav.set("q=path&person=u1&role=role%3Ap1%3Adoor");
    renderPage();

    const panel = panelOf("Priya Nair to Laser Lab · Door");
    expect(within(panel).getByRole("link", { name: /Open Priya Nair/ })).toHaveAttribute("href", "/users/u1");
    expect(within(panel).getByRole("button", { name: "Pin this" })).toBeInTheDocument();
    expect(screen.getByText("The map draws this on a wider screen. The link keeps the question, so it opens there as you left it.")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();
  });
});

function renderPage() {
  return render(<InsightsPage />);
}

const lensRole = (people: number, over: Record<string, unknown> = {}) => ({
  project_id: "p1",
  project_name: "Laser Lab",
  role_key: "trained",
  role_label: "trained",
  people,
  ...over,
});
const usage = (over: Record<string, unknown> = {}) => ({
  sources: { sign_in: { receiving: true }, badge: { receiving: false } },
  held_not_used: [{ project_id: "p1", role_key: "trained", people: 4 }],
  holders_observed: true,
  window_days: 90,
  ...over,
});
const trendsData = {
  window: { from: "2026-08-01", to: "2026-12-31", label: "Monsoon 2026" },
  weekly: [],
  totals: { given: 0, taken_away: 0 },
  requests: { came_in: 0, approved: 0, declined: 0, withdrawn: 0, declined_reasons: [], median_hours_to_decide: null, outside_bundles: [] },
  sending: { median_minutes_approval_to_sent: null, sample: 0 },
  drift: { found: 0, cleared: 0, median_hours_to_clear: null },
  ending_soon: [],
  upkeep: { people_who_left_holding: 0, drift_open: 0, roles_nobody_has: 0, bundles_nothing_new: 0, overlaps: 0 },
};
const lensPill = (name: string) => screen.getByRole("radio", { name });
const params = () => new URLSearchParams(nav.state.search);

describe("lenses", () => {
  it("offers four lenses, always, with People first", () => {
    renderPage();
    expect(within(screen.getByRole("radiogroup", { name: "Lens" })).getAllByRole("radio").map((r) => r.textContent)).toEqual([
      "People",
      "Recent use",
      "Ending soon",
      "A group",
    ]);
    expect(lensPill("People")).toBeChecked();
    expect(screen.queryByLabelText("How far ahead")).not.toBeInTheDocument();
  });

  it("Ending soon lights the roles that lose access, counts them, and holds days in the link", () => {
    insights.lens = {
      data: { kind: "ending", window_days: 30, holders_observed: true, roles: [lensRole(3, { earliest_end: "2026-10-30T12:00:00Z" })] },
      isError: false,
    };
    renderPage();
    fireEvent.click(lensPill("Ending soon"));
    expect(nav.state.search).toBe("lens=ending");
    expect(insights.calls.lens).toEqual({ kind: "ending", days: 30 });

    expect(screen.getByRole("heading", { name: "Access ends in 1 role" })).toBeInTheDocument();
    expect(screen.getByText("The first ends on 30 Oct")).toBeInTheDocument();
    expect(screen.getByText("ends for 3 people, the first on 30 Oct.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open Expiring access/ })).toHaveAttribute("href", "/review/expiring-access");
    // The count sits on the role, which is lit; everything else steps back.
    expect(node("Laser Lab · trained, 3 people lose it within 30 days")).toHaveTextContent("3");
    expect(opacityOf("Laser Lab · trained, 3 people lose it within 30 days")).toBe("1");
    expect(opacityOf(/Metal Shop · welder/)).toBe("0.35");
    expect(screen.getByText("Lit roles have access ending within 30 days. The number is how many people lose it.")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("How far ahead"), { target: { value: "7" } });
    expect(nav.state.search).toBe("lens=ending&days=7");
    expect(insights.calls.lens).toEqual({ kind: "ending", days: 7 });
    fireEvent.change(screen.getByLabelText("How far ahead"), { target: { value: "30" } });
    expect(nav.state.search).toBe("lens=ending");
  });

  it("opens from the link as it was left, and ignores a lens it does not know", () => {
    nav.set("lens=ending&days=90");
    renderPage();
    expect(lensPill("Ending soon")).toBeChecked();
    expect(screen.getByLabelText("How far ahead")).toHaveValue("90");
    expect(insights.calls.lens).toEqual({ kind: "ending", days: 90 });
  });

  it("falls back to People for a lens or a number the link invents", () => {
    nav.set("lens=nonsense&days=5");
    renderPage();
    expect(lensPill("People")).toBeChecked();
  });

  it("says plainly when nothing ends, and when nothing has been read yet", () => {
    insights.lens = { data: { kind: "ending", window_days: 30, holders_observed: true, roles: [] }, isError: false };
    nav.set("lens=ending");
    const { unmount } = renderPage();
    expect(screen.getByRole("heading", { name: "Nothing ends in the next 30 days" })).toBeInTheDocument();
    unmount();

    insights.lens = { data: { kind: "ending", window_days: 30, holders_observed: false, roles: [] }, isError: false };
    renderPage();
    expect(screen.getByText(/hasn't read who holds what from Zitadel yet/)).toBeInTheDocument();
  });

  it("A group starts at people who left, and says the access probably should be revoked in words, not a colour", () => {
    insights.lens = { data: { kind: "group", group: "left", size: 4, holders_observed: true, roles: [lensRole(2)] }, isError: false };
    renderPage();
    fireEvent.click(lensPill("A group"));
    expect(nav.state.search).toBe("lens=group");
    expect(insights.calls.lens).toEqual({ kind: "group", group: "left" });
    expect(screen.getByLabelText("Which group")).toHaveValue("left");
    expect(screen.getByRole("heading", { name: "People who left the makerspace" })).toBeInTheDocument();
    expect(screen.getByText("4 people · still holding 1 role")).toBeInTheDocument();
    expect(screen.getByText("2 of 4 still hold it.")).toBeInTheDocument();
    expect(node("Laser Lab · trained, 2 people in this group")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open People/ })).toHaveAttribute("href", "/users");
  });

  it("A group can be new this term, or a team named in the link", () => {
    insights.lens = { data: { kind: "group", group: "new_term", size: 6, holders_observed: true, roles: [lensRole(5)] }, isError: false };
    nav.set("lens=group");
    renderPage();
    fireEvent.change(screen.getByLabelText("Which group"), { target: { value: "new_term" } });
    expect(nav.state.search).toBe("lens=group&group=new_term");
    expect(insights.calls.lens).toEqual({ kind: "group", group: "new_term" });
    expect(screen.getByRole("heading", { name: "People new this term" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Open People/ })).not.toBeInTheDocument();

    // A team has no name yet: nothing is asked of the server.
    fireEvent.change(screen.getByLabelText("Which group"), { target: { value: "team" } });
    expect(nav.state.search).toBe("lens=group&group=team%3A");
    expect(insights.calls.lens).toBeNull();
    expect(screen.getByRole("heading", { name: "Which team?" })).toBeInTheDocument();

    const name = screen.getByLabelText("Team name");
    fireEvent.change(name, { target: { value: " Design " } });
    fireEvent.blur(name);
    expect(params().get("group")).toBe("team:Design");
    expect(insights.calls.lens).toEqual({ kind: "group", group: "team:Design" });
    expect(screen.getByLabelText("Team name")).toHaveValue("Design");
  });

  it("Recent use lights the roles held but not used, once sign-ins are arriving", () => {
    insights.usage = { data: usage(), isError: false };
    nav.set("lens=use");
    renderPage();
    expect(screen.getByRole("heading", { name: "1 role held but not used in 90 days" })).toBeInTheDocument();
    expect(screen.getByText("is held by 4 people, and none has used it.")).toBeInTheDocument();
    expect(node("Laser Lab · trained, 4 people held but not used")).toBeInTheDocument();
    expect(opacityOf(/Metal Shop · welder/)).toBe("0.35");
    expect(screen.getByText("Lit roles are held by people who haven't used them in 90 days. The number is how many.")).toBeInTheDocument();
  });

  it("Recent use says it fills in once sign-ins arrive, and lights nothing", () => {
    insights.usage = { data: usage({ sources: { sign_in: { receiving: false }, badge: { receiving: false } }, held_not_used: [] }), isError: false };
    nav.set("lens=use");
    renderPage();
    expect(screen.getByText("Recent use fills in once sign-ins arrive.")).toBeInTheDocument();
    expect(opacityOf(/Metal Shop · welder/)).toBe("1");
    expect(screen.queryByText(/held but not used/)).not.toBeInTheDocument();
  });

  it("a lens that could not be read says so and changes nothing", () => {
    insights.lens = { data: undefined, isError: true };
    nav.set("lens=ending");
    renderPage();
    expect(screen.getByText("Syndra couldn't read it just now. Nothing was changed.")).toBeInTheDocument();
  });

  it("is a Select below tablet, with the same four lenses", () => {
    viewport.phone = true;
    viewport.touch = true;
    insights.usage = { data: usage(), isError: false };
    renderPage();
    const select = screen.getByRole("combobox", { name: "Lens" });
    fireEvent.change(select, { target: { value: "use" } });
    expect(nav.state.search).toBe("lens=use");
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Lens: People",
      "Lens: Recent use",
      "Lens: Ending soon",
      "Lens: A group",
    ]);
    expect(screen.getByRole("heading", { name: "1 role held but not used in 90 days" })).toBeInTheDocument();
  });

  it("steps back to People with Back, and when a finding or a question is chosen", () => {
    insights.usage = { data: usage(), isError: false };
    nav.set("lens=use");
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(nav.state.search).toBe("");

    fireEvent.click(lensPill("Ending soon"));
    fireEvent.click(screen.getByRole("button", { name: "2 drift" }));
    expect(nav.state.search).toBe("finding=drift");
    expect(lensPill("People")).toBeChecked();

    fireEvent.click(lensPill("Ending soon"));
    fireEvent.click(node(/Laser Lab · trained/));
    expect(params().get("lens")).toBeNull();
    expect(params().get("q")).toBe("r1");
  });

  it("L moves to the next lens and round again, but not while typing", () => {
    renderPage();
    const order = ["use", "ending", "group", null];
    for (const want of order) {
      fireEvent.keyDown(document.body, { key: "l" });
      expect(params().get("lens")).toBe(want);
    }
    fireEvent.keyDown(screen.getByLabelText("Ask the map"), { key: "L" });
    expect(params().get("lens")).toBeNull();
  });

  it("T opens the date, and again returns to today", () => {
    renderPage();
    fireEvent.keyDown(document.body, { key: "t" });
    expect(screen.getByRole("slider", { name: "Date shown" })).toBeInTheDocument();
    fireEvent.keyDown(document.body, { key: "T" });
    expect(screen.queryByRole("slider", { name: "Date shown" })).not.toBeInTheDocument();
  });

  it("lists T and L among the keys", () => {
    renderPage();
    fireEvent.keyDown(document.body, { key: "?" });
    const keys = within(screen.getByRole("dialog", { name: "Keys" }));
    expect(keys.getByText("Choose a date")).toBeInTheDocument();
    expect(keys.getByText("Next lens")).toBeInTheDocument();
  });

  it("on an earlier date, only People can be chosen, and the others come back with today", () => {
    nav.set("lens=ending&asof=2020-02-29");
    insights.asof = { data: { kind: "none", first_day: "2026-10-16" }, isError: false };
    renderPage();
    expect(lensPill("People")).toBeChecked();
    for (const name of ["Recent use", "Ending soon", "A group"]) expect(lensPill(name)).toBeDisabled();
    expect(screen.getByText("Other lenses read today's access.")).toBeInTheDocument();
    expect(screen.queryByLabelText("How far ahead")).not.toBeInTheDocument();
    expect(insights.calls.lens).toBeNull();

    fireEvent.keyDown(document.body, { key: "l" });
    expect(params().get("lens")).toBe("ending");

    fireEvent.click(screen.getByRole("button", { name: "Back to now" }));
    expect(lensPill("Ending soon")).toBeChecked();
  });

  it("on Trends, Held but not used lists roles and links to the lens", () => {
    insights.usage = { data: usage(), isError: false };
    insights.trends = { ...insights.trends, data: trendsData };
    nav.set("tab=trends");
    renderPage();
    const card = screen.getByText("Held but not used").closest(".card") as HTMLElement;
    expect(within(card).getByText("Not used in 90 days")).toBeInTheDocument();
    expect(within(card).getByText("Laser Lab · trained")).toBeInTheDocument();
    expect(within(card).getByText("4 people")).toBeInTheDocument();
    expect(within(card).getByRole("link", { name: /Show on the map/ })).toHaveAttribute("href", "/insights?lens=use");
  });

  it("on Trends, says recent use fills in once sign-ins arrive, and lists nothing", () => {
    insights.usage = { data: usage({ sources: { sign_in: { receiving: false }, badge: { receiving: false } }, held_not_used: [] }), isError: false };
    insights.trends = { ...insights.trends, data: trendsData };
    nav.set("tab=trends");
    renderPage();
    expect(screen.getByText("Recent use fills in once sign-ins arrive.")).toBeInTheDocument();
  });
});

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
  calls: {} as { who?: [string | null, string | null]; change?: unknown },
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
    expect(screen.getByText("Trends are being built.")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Access map" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Map" }));
    expect(nav.state.search).toBe("");
    expect(screen.getByRole("group", { name: "Access map" })).toBeInTheDocument();
  });

  it("opens straight on Trends from the link", () => {
    nav.set("tab=trends");
    renderPage();
    expect(screen.getByRole("button", { name: "Trends" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByText("They’ll show here.")).toBeInTheDocument();
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

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
    expect(screen.getByText("Nobody has Metal Shop · welder.")).toBeInTheDocument();
  });

  it("only counts rows that wait, and hides a chip with nothing behind it", () => {
    reads.drift = [];
    reads.catalog = [role("p1", "trained", 5), role("p2", "welder", 3)];
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
    expect(screen.getByRole("button", { name: "Pin" })).toBeInTheDocument();
  });
});

function renderPage() {
  return render(<InsightsPage />);
}

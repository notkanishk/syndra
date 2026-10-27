// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ConnectedSystemsPage from "@/app/system/targets/page";
import type { TargetSummary } from "@/lib/queries/useTargets";

/**
 * The index exists for the deployment that has registered NOTHING. That case is
 * the one worth testing hardest: before this page it produced no row, no page
 * and no sentence anywhere in the product, and an operator reading that silence
 * concluded the add-on platform had not shipped.
 */

const state = {
  targets: [] as TargetSummary[],
  isLoading: false,
  error: null as unknown,
  // What the system's own page reads say, one per hook. `undefined` data is a
  // read that has not answered (or failed): the row must not invent a count.
  health: undefined as Record<string, unknown> | undefined,
  inventory: undefined as Record<string, unknown> | undefined,
  findings: undefined as unknown[] | undefined,
};

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useTargets: () => ({
      data: state.targets,
      isLoading: state.isLoading,
      error: state.error,
      refetch: vi.fn(),
    }),
    useTargetHealth: () => ({ data: state.health, isLoading: false, isError: false }),
    useTargetInventory: () => ({ data: state.inventory, isLoading: false, isError: false }),
    useMergeFindings: () => ({ data: state.findings, isLoading: false, isError: false }),
  };
});

function target(over: Partial<TargetSummary> = {}): TargetSummary {
  return {
    target: "truenas",
    registered: true,
    auth_mode: "derived",
    callable: true,
    operations: [
      { id: "account.provision", scope: "member", confirm: false, available: true },
      { id: "account.release", scope: "member", confirm: true, available: true },
    ],
    circuit_open: false,
    status: "answering",
    health: { status: "online", recent: ["answered", "answered", "answered"], checked_at: "2026-10-14T09:41:00Z" },
    project: { id: "p1", name: "Makerspace" },
    ...over,
  };
}

beforeEach(() => {
  state.targets = [];
  state.isLoading = false;
  state.error = null;
  state.health = undefined;
  state.inventory = undefined;
  state.findings = undefined;
});

describe("connected systems", () => {
  it("says so, in words, when the deployment registered nothing", () => {
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("No system is connected.")).toBeTruthy();
    // And says what would connect one, because the operator's next question is
    // not "is it empty" but "what do I do about it".
    expect(screen.getByText(/ADDON_TARGETS/)).toBeTruthy();
  });

  it("does not claim a count it does not have", () => {
    render(<ConnectedSystemsPage />);
    expect(screen.queryByText(/connected$/)).toBeNull();
  });

  it("lists a registered target under the board's columns, and not the old operation count", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);

    expect(screen.getByRole("link", { name: "TrueNAS" })).toBeTruthy();
    expect(screen.getByText("1 system connected")).toBeTruthy();
    expect(screen.getByText("Answering")).toBeTruthy();
    for (const heading of ["Status", "Needs you", "People"]) expect(screen.getByText(heading)).toBeTruthy();
    // A count of add-on operations is nothing an operator acts on.
    expect(screen.queryByText("Things it can do")).toBeNull();
  });

  it("lists systems A to Z, whatever order they were registered in", () => {
    state.targets = [target({ target: "truenas" }), target({ target: "door", callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    const links = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    // "door" sorts before "truenas" by its label.
    expect(links.indexOf("/system/targets/door")).toBeLessThan(links.indexOf("/system/targets/truenas"));
  });

  it("names the product and version the system reports, and the project it serves", () => {
    state.targets = [target()];
    state.health = { product: "TrueNAS SCALE", product_version: "TrueNAS-24.10.2" };
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("TrueNAS SCALE 24.10.2")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Makerspace" }).getAttribute("href")).toBe("/projects/p1");
  });

  it("says it has not read the product yet, rather than leaving the line empty", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);
    expect(screen.getByText("Not read yet")).toBeTruthy();
  });

  // People is the same number the system's own page shows as "Accounts Syndra
  // manages": one read, so the two pages cannot disagree.
  it("counts the people with an account Syndra manages there, from the system's own inventory", () => {
    state.targets = [target()];
    state.inventory = { bound: 48, unmanaged: [], current: true };
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("48")).toBeTruthy();
    expect(screen.queryByText("last known")).toBeNull();
  });

  it("keeps the last count and labels it once the system stops answering", () => {
    state.targets = [
      target({
        status: "offline",
        health: { status: "offline", recent: ["missed", "missed", "missed"], reachable_since: "2026-10-14T08:33:00Z" },
      }),
    ];
    state.inventory = { bound: 48, unmanaged: [], current: true };
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("48")).toBeTruthy();
    expect(screen.getByText("last known")).toBeTruthy();
    expect(screen.getByText("Unknown until it answers")).toBeTruthy();
  });

  it("shows a dash, never 0, when the count has not been read", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);
    expect(screen.queryByText("0")).toBeNull();
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  describe("Needs you", () => {
    it("links each thing that is waiting to the part of the system's page that handles it", () => {
      state.targets = [target()];
      state.inventory = { bound: 48, unmanaged: [{ username: "a" }, { username: "b" }, { username: "c" }], current: true };
      state.findings = [{ id: "f1" }];
      render(<ConnectedSystemsPage />);

      expect(screen.getByRole("link", { name: "3 accounts Syndra doesn't manage" }).getAttribute("href")).toBe(
        "/system/targets/truenas#people",
      );
      expect(screen.getByRole("link", { name: "1 value somebody changed there" }).getAttribute("href")).toBe(
        "/system/targets/truenas#waiting",
      );
      expect(screen.queryByText("Nothing waiting")).toBeNull();
    });

    it("counts a tampered change log and a disputed account as decisions", () => {
      state.targets = [target()];
      state.inventory = { bound: 3, unmanaged: [], current: true };
      state.findings = [];
      state.health = { log_anchor: { violation_reason: "edited" }, binding_conflicts: [{ id: "c1" }] };
      render(<ConnectedSystemsPage />);

      expect(screen.getByRole("link", { name: "2 things need a decision" })).toBeTruthy();
    });

    it("says nothing is waiting, so an empty cell never reads as unknown", () => {
      state.targets = [target()];
      state.inventory = { bound: 3, unmanaged: [], current: true };
      state.findings = [];
      render(<ConnectedSystemsPage />);

      expect(screen.getByText("Nothing waiting")).toBeTruthy();
    });

    it("says unknown until it answers, never nothing waiting, for a system that is not answering", () => {
      state.targets = [target({ circuit_open: true, status: "waiting_to_retry" })];
      state.findings = [];
      render(<ConnectedSystemsPage />);

      expect(screen.getByText("Unknown until it answers")).toBeTruthy();
      expect(screen.queryByText("Nothing waiting")).toBeNull();
    });
  });

  it("says when the system last answered, under its status", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);
    expect(screen.getByText(/^Answered /)).toBeTruthy();
  });

  // A-07 §22: the API grows "Last answered", which freshness rests on. Without it the row says
  // what it said before.
  it("says when a system last answered, from the field the API carries", () => {
    const at = new Date(Date.now() - 3 * 3_600_000).toISOString();
    state.targets = [
      target({
        status: "offline",
        health: {
          status: "offline",
          recent: ["missed", "missed", "missed"],
          reachable_since: "2026-10-14T08:33:00Z",
          last_answered_at: at,
        },
      }),
    ];
    render(<ConnectedSystemsPage />);
    expect(screen.getByText(/^Last answered /)).toHaveTextContent(/^Last answered 3 .*ago$/);
  });

  it("says nothing of a last answer for a system that has never given one", () => {
    state.targets = [target({ health: { status: "unknown", recent: [], last_answered_at: null } })];
    render(<ConnectedSystemsPage />);
    expect(screen.queryByText(/^Last answered /)).toBeNull();
  });

  // Registration is a deployment fact and a manifest is a runtime one. Showing
  // `0` for a target that has never answered would be a claim about the TARGET
  // — that it can do nothing — rather than about Syndra not having asked.
  it("shows dashes, not zero, for a target that has never answered", () => {
    state.targets = [target({ callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("No answer yet")).toBeTruthy();
    expect(screen.queryByText("0")).toBeNull();
    // Needs you and People: Syndra has not read it.
    expect(screen.getAllByText("—")).toHaveLength(2);
    expect(screen.queryByText("Nothing waiting")).toBeNull();
    expect(screen.getByText(/usually clears by itself/)).toBeTruthy();
  });

  // A secret that will not load is a fault on THIS side. Reporting it as the
  // target being unreachable sends an operator to the wrong machine.
  it("separates a transport fault from a target that is merely quiet", () => {
    state.targets = [target({ transport_status: "error", transport_error: "no such file", status: "cannot_connect" })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Cannot connect")).toBeTruthy();
    expect(screen.queryByText("Answering")).toBeNull();
    expect(screen.getByText(/Look at the Syndra server, not at TrueNAS/)).toBeTruthy();
  });

  // A-01: the breaker is "Waiting to retry", never "paused", which is only
  // ever a person's choice.
  it("reports the breaker as waiting to retry, and where to look", () => {
    state.targets = [target({ circuit_open: true, status: "waiting_to_retry" })];
    const { container } = render(<ConnectedSystemsPage />);

    expect(screen.getByText("Waiting to retry")).toBeTruthy();
    expect(screen.getByText(/look at the Syndra server/)).toBeTruthy();
    expect(container.textContent).not.toMatch(/paused/i);
  });

  it("calls a system offline only after three misses, with when it began and the pips", () => {
    state.targets = [
      target({
        status: "offline",
        health: { status: "offline", recent: ["missed", "missed", "missed"], reachable_since: "2026-10-14T08:33:00Z" },
      }),
    ];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Offline")).toBeTruthy();
    expect(screen.getByText(/^Since \d\d:\d\d$/)).toBeTruthy();
    expect(screen.getByRole("img", { name: "Last 3 checks: missed, missed, missed" })).toBeTruthy();
  });

  it("keeps a missed check green, with a faint line and the pips", () => {
    state.targets = [
      target({ health: { status: "online", recent: ["answered", "answered", "missed"], missed_since: "2026-10-14T09:40:00Z" } }),
    ];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Answering")).toBeTruthy();
    expect(screen.getByText(/^Missed 1 check, \d\d:\d\d$/)).toBeTruthy();
    expect(screen.getByRole("img", { name: "Last 3 checks: answered, answered, missed" })).toBeTruthy();
  });

  it("shows no pips on a healthy row", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);
    expect(screen.queryByRole("img", { name: /Last 3 checks/ })).toBeNull();
  });

  it("says checking, not offline, when Syndra has just restarted", () => {
    state.targets = [target({ status: "checking", health: { status: "unknown", recent: [], started_at: "2026-10-14T09:38:00Z" } })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Checking")).toBeTruthy();
    expect(screen.getByText(/^No check since restart, \d\d:\d\d$/)).toBeTruthy();
    expect(screen.getByRole("img", { name: "Last 3 checks: no check yet, no check yet, no check yet" })).toBeTruthy();
  });

  it("names no clock when the API omits the start time", () => {
    state.targets = [target({ status: "checking", health: { status: "unknown", recent: [] } })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Not checked yet")).toBeTruthy();
    expect(screen.queryByText(/No check since restart/)).toBeNull();
  });

  // 6.5: the project, read-only; none chosen is the row's Needs you.
  it("shows the project it serves, or the way to choose one", () => {
    state.targets = [target()];
    const { unmount } = render(<ConnectedSystemsPage />);
    expect(screen.getByText("Zitadel project")).toBeTruthy();
    unmount();

    state.targets = [target({ project: null })];
    render(<ConnectedSystemsPage />);
    expect(screen.getByText("No Zitadel project yet")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Choose its Zitadel project" }).getAttribute("href")).toBe("/system/targets/truenas");
  });

  // Q-01 (14 Oct): a fault is one Notice above the page, said once; the row
  // keeps its dot and word.
  it("puts a system that stopped answering in one amber Notice above the page, and keeps the row's word", () => {
    state.targets = [
      target({
        status: "offline",
        health: { status: "offline", recent: ["missed", "missed", "missed"], reachable_since: "2026-10-14T08:33:00Z" },
      }),
    ];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("TrueNAS isn't answering.")).toBeTruthy();
    expect(screen.getByText(/Look at TrueNAS, not at Syndra/)).toBeTruthy();
    expect(screen.getByText("Offline")).toBeTruthy();
  });

  it("gives an answering system no Notice", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

/**
 * The tone on "registered, no manifest read yet".
 *
 * It shipped amber, and amber in this system is a deadline or a broken
 * assumption. This is neither: it is where every add-on starts, and it resolves
 * on its own within a refresh interval. Spending amber on the ordinary first
 * minute of something's life is how amber stops meaning anything on the screens
 * where it does — the unrecorded key expiry two cards up, for one.
 */
describe("the tone a reading wears", () => {
  const toneOf = (label: string) =>
    screen.getByText(label).className.match(/text-(muted|warn-text|danger-text|healthy)/)?.[1];

  it("does not spend amber on a target that has simply not answered yet", () => {
    state.targets = [target({ callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("No answer yet")).toBe("muted");
  });

  it("keeps amber for the reading that is a real fault on this host", () => {
    state.targets = [target({ transport_status: "error", transport_error: "no such file", status: "cannot_connect" })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("Cannot connect")).toBe("danger-text");
  });

  it("keeps amber for a breaker somebody has to act on", () => {
    state.targets = [target({ circuit_open: true, status: "waiting_to_retry" })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("Waiting to retry")).toBe("warn-text");
  });
});

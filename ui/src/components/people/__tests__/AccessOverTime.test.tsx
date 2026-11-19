// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AccessOverTime } from "@/components/people/AccessOverTime";
import { CAPTION, CUT_SHORT, NOTHING, READ_FAILED } from "@/components/people/accessRuns";
import type { AccessHistory, RoleHistory } from "@/lib/queries/useUsers";
import { inZone } from "@/test-utils/zone";

/**
 * Access over time (A-24 §9 vivid, Q11): a line per role on this year's axis, each one sentence for a
 * screen reader, A to Z by what it is in; ink held, grey ended, dashed amber the time left; a start no
 * record gives drawn from the edge and said; every day the makerspace's, never the browser's.
 */
const state = vi.hoisted(() => ({
  history: { data: undefined as AccessHistory | undefined, isPending: false, isError: false, error: null as unknown, refetch: () => {} },
  asked: [] as string[],
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useAccessHistory: (userId: string, since: string) => {
    state.asked.push(`${userId}@${since}`);
    return state.history;
  },
}));
vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({
    data: [
      { name: "Spring", start: "2026-01-12", end: "2026-05-29" },
      { name: "Monsoon term", start: "2026-08-03", end: "2026-12-04" },
    ],
  }),
}));
vi.mock("@/components/names/shownNames", () => ({
  useShownNames: () => ({ role: (_p: string, key: string) => key.charAt(0).toUpperCase() + key.slice(1) }),
}));

const role = (over: Partial<RoleHistory>): RoleHistory => ({
  project_id: "p",
  project_name: "Laser cutting",
  project_name_resolved: true,
  role_key: "operator",
  standing: "held",
  runs: [],
  ...over,
});

function show(roles: RoleHistory[], truncated = false) {
  state.history = { data: { since: "2026-01-01", truncated, roles }, isPending: false, isError: false, error: null, refetch: () => {} };
  return render(<AccessOverTime userId="u1" lines={2} />);
}

const lineOf = (text: string) => screen.getByText(text).closest("li")!;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-11-19T06:00:00Z") });
  state.asked = [];
});
afterEach(() => {
  vi.useRealTimers();
  inZone("Asia/Kolkata");
});

describe("Access over time", () => {
  it("reads every line as one sentence, A to Z by what it is in, and says the dates are Syndra's", () => {
    show([
      role({ project_name: "Wood shop", role_key: "member", standing: "not_held", runs: [{ from: "2026-03-03T04:00:00Z", to: "2026-05-31T04:00:00Z" }] }),
      role({ runs: [{ from: "2026-08-03T04:00:00Z", to: null }], ends: "2026-12-12T18:29:59Z" }),
      role({ project_name: "3D printing", role_key: "member", runs: [{ from: "2025-09-01T04:00:00Z", to: null }] }),
      role({ role_key: "trained", standing: "not_held", runs: [{ from: null, to: "2026-08-14T04:00:00Z" }] }),
    ]);
    const lines = screen.getAllByRole("listitem").filter((li) => li.hasAttribute("data-line"));
    expect(lines.map((li) => li.querySelector(".sr-only")!.textContent)).toEqual([
      "3D printing · Member, given before this year, held today",
      "Laser cutting · Operator, given 3 Aug, held today, ends 12 Dec",
      "Laser cutting · Trained, start not on record, ended 14 Aug",
      "Wood shop · Member, given 3 Mar, ended 31 May",
    ]);
    expect(screen.getByText(CAPTION)).toBeInTheDocument();
    expect(state.asked).toEqual(["u1@2026-01-01"]);
  });

  it("draws held in ink, ended in grey, and the time left before an end dashed, only for a role held today", () => {
    show([
      role({ runs: [{ from: "2026-08-03T04:00:00Z", to: null }], ends: "2026-12-12T18:29:59Z" }),
      role({ role_key: "lathe", standing: "not_held", runs: [{ from: "2026-02-02T04:00:00Z", to: null }], ends: "2026-12-12T18:29:59Z" }),
      role({ role_key: "trained", standing: "not_held", runs: [{ from: "2026-02-02T04:00:00Z", to: "2026-08-14T04:00:00Z" }] }),
    ]);
    const held = lineOf("Laser cutting · Operator, given 3 Aug, held today, ends 12 Dec");
    expect(held.querySelector("[data-run]")!.getAttribute("data-run")).toBe("held");
    expect(held.querySelector("[data-ahead]")).not.toBeNull();
    // Given, never seen in Zitadel: grey to today, its standing said, and no time left drawn.
    const missing = lineOf("Laser cutting · Lathe, given 2 Feb, not in Zitadel, ends 12 Dec");
    expect(missing.querySelector("[data-run]")!.getAttribute("data-run")).toBe("ended");
    expect(missing.querySelector("[data-ahead]")).toBeNull();
    expect(screen.getByText("Since 2 Feb · Not in Zitadel · ends 12 Dec")).toBeInTheDocument();
    const ended = lineOf("Laser cutting · Trained, given 2 Feb, ended 14 Aug");
    expect(ended.querySelector("[data-run]")!.getAttribute("data-run")).toBe("ended");
    expect(screen.getByText("2 Feb to 14 Aug")).toBeInTheDocument();
  });

  it("draws a start no record gives from the year's edge with its tick, and never says a day for it", () => {
    show([
      role({ runs: [{ from: null, to: null }] }),
      role({ role_key: "lathe", runs: [{ from: "2025-06-01T04:00:00Z", to: null }] }),
      role({ role_key: "kiln", runs: [{ from: "2026-04-04T04:00:00Z", to: null }] }),
    ]);
    const unknown = lineOf("Laser cutting · Operator, start not on record, held today");
    expect(unknown.querySelector("[data-run]")!.hasAttribute("data-clipped")).toBe(true);
    expect(screen.getByText("Start not on record")).toBeInTheDocument();
    expect(lineOf("Laser cutting · Lathe, given before this year, held today").querySelector("[data-clipped]")).not.toBeNull();
    expect(screen.getByText("Since before this year")).toBeInTheDocument();
    expect(lineOf("Laser cutting · Kiln, given 4 Apr, held today").querySelector("[data-clipped]")).toBeNull();
  });

  it("counts days in the makerspace's zone, the year's first day included", () => {
    // 04:00 UTC on 3 Aug is still 2 Aug in Los Angeles, and already 3 Aug where this test runs (Kolkata).
    inZone("America/Los_Angeles");
    show([role({ runs: [{ from: "2026-08-03T04:00:00Z", to: null }] })]);
    expect(screen.getByText("Laser cutting · Operator, given 2 Aug, held today")).toBeInTheDocument();
  });

  it("asks for the year the makerspace is in, not the browser's", () => {
    // 31 Dec 20:00 UTC is already 1 Jan 2027 in Kolkata.
    vi.setSystemTime(new Date("2026-12-31T20:00:00Z"));
    inZone("Asia/Kolkata");
    show([]);
    expect(state.asked).toEqual(["u1@2027-01-01"]);
    inZone("UTC");
    state.asked = [];
    show([]);
    expect(state.asked).toEqual(["u1@2026-01-01"]);
  });

  it("holds its height while the read is out: a line per role held today", () => {
    state.history = { data: undefined, isPending: true, isError: false, error: null, refetch: () => {} };
    render(<AccessOverTime userId="u1" lines={3} />);
    expect(screen.getByLabelText("Loading").children).toHaveLength(3);
  });

  it("says a failed read failed, and never that nothing was given", () => {
    state.history = { data: undefined, isPending: false, isError: true, error: new Error("down"), refetch: () => {} };
    render(<AccessOverTime userId="u1" lines={1} />);
    expect(screen.getByText(READ_FAILED)).toBeInTheDocument();
    expect(screen.queryByText(NOTHING)).not.toBeInTheDocument();
  });

  it("says nothing was given, and that a cut-short read is cut short", () => {
    show([], true);
    expect(screen.getByText(NOTHING)).toBeInTheDocument();
    expect(screen.getByText(`${CAPTION} ${CUT_SHORT}`)).toBeInTheDocument();
  });

  it("marks the terms and today on the axis", () => {
    show([role({ runs: [{ from: "2026-08-03T04:00:00Z", to: null }] })]);
    const axis = document.querySelector("[data-axis]")!;
    expect(axis.textContent).toContain("Spring term");
    expect(axis.textContent).toContain("Monsoon term");
    expect(axis.textContent).toContain("Today");
  });
});

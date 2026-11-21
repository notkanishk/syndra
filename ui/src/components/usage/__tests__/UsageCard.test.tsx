// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { STORAGE_USAGE_STEP } from "@/components/kinds/storage/captions";
import * as copy from "@/components/usage/copy";
import { TargetStats } from "@/components/usage/TargetStats";
import { insightsUsageHref } from "@/components/usage/InsightsUsage";
import { HomeMonth, UsageCard, ViewUsage } from "@/components/usage/UsageCard";
import { formatRelative } from "@/lib/format";
import type { StatsGap, StatsMetric } from "@/lib/queries/useStats";
import { UiViewProvider } from "@/lib/ui-view";
import { dayWords, formatValue } from "@/lib/usage";
import { motionInTests } from "@/test-utils/motion";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { ago, GAP, GIB, homeBytes, sessions, targetStats, view, windows } from "./fixtures";

const withQuery = (ui: ReactElement, advanced = false) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <UiViewProvider isOperator initialView={advanced ? "advanced" : "basic"}>
      {ui}
    </UiViewProvider>
  </QueryClientProvider>
);
afterEach(() => {
  vi.unstubAllGlobals();
  motionInTests(false);
});

const titles = (root: Element) => Array.from(root.querySelectorAll("svg title")).map((t) => t.textContent ?? "");
const rows = (root: Element) => Array.from(root.querySelectorAll("[data-row]"));
/** Each row's figure, its first line: the second line and the axis are figures too, beside it. */
const figures = (root: Element) => Array.from(root.querySelectorAll("[data-row] [data-figure]:first-child")).map((f) => f.textContent);
const bands = () => screen.queryAllByRole("img");
/** A level with no limit of its own, so the usage card is not the kind's meter. */
const level = (over: Partial<StatsMetric> = {}) => homeBytes({ against: undefined, ...over });
/** The buffer gap: the add-on's days before its first collection, from Syndra's first read of its whole buffer. */
const buffer = (began: number): StatsGap => ({ start: ago(began + 90), end: ago(began), kind: "began", reason: "Not collected yet: counting began on a day" });
const openFold = () => fireEvent.click(screen.getByRole("button", { name: copy.FOLD_HEADING }));

describe("the usage card: a row", () => {
  it("sums a counter over both windows, one row per series named by its value, and shows a level's latest with its day", () => {
    const { container } = render(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[level(), sessions()]} />);
    expect(figures(container)).toEqual([`18.4 GiB on ${dayWords(ago(1))}`, "7 in the last 30 days", "0 in the last 30 days"]);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
    // A row's name is the add-on's value: no label, no quotes (those stay in sentences).
    expect(screen.getByText("ring")).toBeInTheDocument();
    expect(screen.getByText("mat")).toBeInTheDocument();
    expect(screen.queryByText(/Area: “ring”/)).toBeNull();
  });

  it("is one grid row: name, band, figure in the 200 · band · 240 ledger, and the figure in the stat face", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    for (const row of rows(container)) {
      expect(row.className).toContain("@tablet:grid-cols-[200px_minmax(0,1fr)_240px]");
      expect(row.children).toHaveLength(3);
      expect(row.children[2].className).toContain("@tablet:items-end");
    }
    const value = within(rows(container)[0] as HTMLElement).getByText("7");
    expect(value.className).toContain("font-display");
    // The figure step (D-1 §3: 24px, 600, "I like big number"), its weight from the token.
    expect(value.className).toContain("text-figure");
    // The card is the container the row answers to: it stacks under the tablet step of its own width.
    expect(screen.getByRole("group")).toHaveClass("@container");
  });

  // The board's own regression: a guard nobody sets is a band that never mounts.
  it("mounts every row's band", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[level(), sessions()]} />);
    const drawn = rows(container);
    expect(drawn).toHaveLength(3);
    for (const row of drawn) expect(row.children[1].querySelector('svg[role="img"]'), row.textContent ?? "").not.toBeNull();
  });

  it("puts a one-series metric's label in the row's name seat, and a several-row metric's on its own line", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[level(), sessions()]} />);
    const [home, ring] = rows(container);
    expect(within(home.children[0] as HTMLElement).getByRole("heading", { name: "Home folder used" })).toBeInTheDocument();
    expect(within(ring.children[0] as HTMLElement).queryByRole("heading")).toBeNull();
    expect(screen.getByRole("heading", { name: "Sessions" }).closest("[data-row]")).toBeNull();
  });

  it("has one axis per section, with today at rest in its readout seat", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[level(), sessions()]} />);
    for (const name of ["Home folder used", "Sessions"]) {
      const section = screen.getByRole("region", { name });
      const seats = section.querySelectorAll('[aria-live="polite"]');
      expect(seats, name).toHaveLength(1);
      expect(seats[0].textContent).toBe(dayWords(ago(0)));
    }
  });

  it("reads a day into the section's one readout, led by the row's name, by hover and by ← →", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    const ring = bands()[0];
    const readout = () => container.querySelector('[aria-live="polite"]')!.textContent;
    fireEvent.pointerEnter(ring.querySelectorAll("rect:not([data-column])")[28]);
    expect(readout()).toBe(copy.readoutNamed("ring", copy.dayValue(dayWords(ago(1)), "3")));
    expect(ring.parentElement!.querySelector("[data-read]")).not.toBeNull();
    fireEvent.pointerLeave(ring);
    expect(readout()).toBe(dayWords(ago(0)));
    fireEvent.focus(ring);
    fireEvent.keyDown(ring, { key: "ArrowLeft" });
    expect(readout()).toBe(copy.readoutNamed("ring", copy.dayValue(dayWords(ago(1)), "3")));
  });

  it("ends the first and the last column inside the band's edges", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [{ dimensions: { area: "ring" }, sum_30d: 2, sum_12m: 2, days: [{ day: ago(29), value: 1 }, { day: ago(0), value: 1 }], months: [] }] })]} />);
    const svg = bands()[0];
    const width = Number(svg.getAttribute("width"));
    const columns = Array.from(svg.querySelectorAll("[data-column]"));
    expect(columns).toHaveLength(30);
    const x = (c: Element) => Number(c.getAttribute("x"));
    const w = (c: Element) => Number(c.getAttribute("width"));
    expect(x(columns[0])).toBeGreaterThanOrEqual(-0.01);
    expect(x(columns[0])).toBeLessThan(1);
    expect(x(columns[29]) + w(columns[29])).toBeLessThanOrEqual(width + 0.01);
    expect(x(columns[29]) + w(columns[29])).toBeGreaterThan(width - 1);
    // A real 0 is a 1.5px stub, never nothing.
    expect(columns.slice(1, 29).every((c) => c.getAttribute("height") === "1.5")).toBe(true);
  });
});

describe("a level's change over the record", () => {
  const changing = (a: number, b: number) =>
    level({ series: [{ dimensions: {}, latest: { day: ago(1), value: b }, days: [{ day: ago(6), value: a }, { day: ago(1), value: b }] }] });
  it("is up, down, or unchanged within rounding, from the first counted day", () => {
    const { rerender } = render(<UsageCard title="t" windows={windows()} metrics={[changing(18 * GIB, 18.4 * GIB)]} />);
    expect(screen.getByText(copy.changeUp(formatValue(18.4 * GIB - 18 * GIB, "bytes"), dayWords(ago(6))))).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={windows()} metrics={[changing(18.4 * GIB, 18 * GIB)]} />);
    expect(screen.getByText(copy.changeDown(formatValue(18.4 * GIB - 18 * GIB, "bytes"), dayWords(ago(6))))).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={windows()} metrics={[changing(18.4 * GIB, 18.4 * GIB + 1)]} />);
    expect(screen.getByText(copy.changeNone(dayWords(ago(6))))).toBeInTheDocument();
  });
});

describe("what wasn't counted: the card's one fold", () => {
  const gapped = [sessions({ gaps: [GAP] })];

  it("starts closed, and opens to the add-on's reason, quoted, and that a day with no line is not a zero", () => {
    render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    const toggle = screen.getByRole("button", { name: copy.FOLD_HEADING });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText(/Auditing is off in this area/)).toBeNull();
    openFold();
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("list", { name: copy.GAPS_HEADING })).toBeInTheDocument();
    expect(screen.getByText(/Area: “ring”: “Auditing is off in this area”/)).toBeInTheDocument();
    expect(screen.getByText(copy.GAPS_NOTE)).toBeInTheDocument();
  });

  it("is one per card, its lines merged across metrics where range and reason are the same", () => {
    const off = { start: ago(8), end: ago(7), kind: "unread" as const, reason: "The NAS was off" };
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ gaps: [off] }), level({ gaps: [off] })]} />);
    expect(container.querySelectorAll("[data-fold]")).toHaveLength(1);
    openFold();
    expect(screen.getAllByText(`${dayWords(ago(8))}: “The NAS was off”`)).toHaveLength(1);
  });

  it("lists gaps within the widest window the block shows", () => {
    const old = { start: ago(60), end: ago(58), dimensions: { area: "ring" }, kind: "unobserved" as const, reason: "Auditing is off in this area" };
    const { unmount } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ gaps: [old] })]} />);
    openFold();
    expect(screen.getByText(/Area: “ring”: “Auditing is off in this area”/)).toBeInTheDocument();
    unmount();
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[level({ gaps: [old] })]} />);
    expect(container.querySelector("[data-fold]")).toBeNull();
  });

  it("is not there when there is nothing to say", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    expect(container.querySelector("[data-fold]")).toBeNull();
  });

  it("hatches the days and breaks the columns and the line: no gap day is a zero, and no gap day has a value", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    const [ring, mat] = bands();
    expect(ring.querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(ring.querySelectorAll("[data-column]")).toHaveLength(27);
    expect(titles(container).filter((t) => t.endsWith("couldn't count"))).toHaveLength(3);
    // The other series is not named by the gap: not hatched, every day a column.
    expect(mat.querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(mat.querySelectorAll("[data-column]")).toHaveLength(30);
    const ringTitles = titles(ring);
    for (const day of [ago(8), ago(7), ago(6)]) expect(ringTitles, day).toContain(copy.dayGap(dayWords(day)));
    expect(ringTitles).not.toContain(copy.dayGap(dayWords(ago(5))));
  });

  it("breaks a level's line at a gap, and at a day with no sample", () => {
    const gauge = level({ gaps: [{ start: ago(8), end: ago(5), kind: "unread", reason: "The NAS was off" }], series: [{ dimensions: {}, latest: { day: ago(1), value: 5 }, days: [{ day: ago(12), value: 4 }, { day: ago(10), value: 4 }, { day: ago(3), value: 5 }, { day: ago(1), value: 5 }] }] });
    render(<UsageCard title="t" windows={windows()} metrics={[gauge]} />);
    const svg = bands()[0];
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(0);
    expect(svg.querySelectorAll("[data-dot]")).toHaveLength(4);
    expect(svg.querySelectorAll("[data-gap]")).toHaveLength(1);
  });

  it("describes the hatching to a screen reader", () => {
    render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    expect(bands()[0]).toHaveAccessibleName(/Days that could not be counted are hatched/);
    expect(bands()[1]).not.toHaveAccessibleName(/hatched/);
  });
});

describe("the states (A-23 part 2)", () => {
  it("not counted yet: one row per metric, its label, an empty band seat, no axis, and its step", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ watermark: null, series: [] }), level({ watermark: null, series: [] })]} />);
    expect(rows(container)).toHaveLength(2);
    expect(screen.getAllByText(copy.NOT_COUNTED)).toHaveLength(2);
    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelector('[aria-live="polite"]')).toBeNull();
    expect(screen.getByText(copy.NOT_COUNTED_NEXT)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("day one: the window's faint baseline, a tick where the record starts, one column, today blank, and when counting began", () => {
    const m = sessions({ gaps: [buffer(1)], began: ago(1), counted_through: ago(1), series: [{ dimensions: { area: "ring" }, sum_30d: 5, sum_12m: 5, days: [{ day: ago(1), value: 5 }], months: [] }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const svg = bands()[0];
    expect(svg.querySelector("[data-pre]")).not.toBeNull();
    // --line is a 6-8% tint that vanishes on real data: the days before the record read as a baseline in --line-strong.
    expect(svg.querySelector("[data-pre]")!.getAttribute("stroke")).toBe("var(--line-strong)");
    expect(svg.querySelector("[data-tick]")).not.toBeNull();
    expect(svg.querySelectorAll("[data-column]")).toHaveLength(1);
    // Before the record is never hatched and never a zero.
    expect(svg.querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(titles(svg).filter((t) => t.endsWith("before counting began"))).toHaveLength(28);
    expect(titles(svg)).toContain(copy.dayPending(dayWords(ago(0))));
    expect(figures(container)).toEqual(["5 in the last 2 days"]);
    expect(screen.queryByText(/12 months/)).toBeNull();
    expect(screen.getByText(copy.countingBegan(dayWords(ago(1))))).toBeInTheDocument();
    // The days before the record are not a gap: nothing to fold.
    expect(container.querySelector("[data-fold]")).toBeNull();
  });

  it("ten days in: the last 10 days, and a level's change since its first counted day", () => {
    const m = level({ gaps: [buffer(9)], began: ago(9), series: [{ dimensions: {}, latest: { day: ago(1), value: 3 * GIB }, days: Array.from({ length: 9 }, (_, i) => ({ day: ago(9 - i), value: (2 + i / 8) * GIB })) }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m, sessions({ gaps: [buffer(9)], began: ago(9), series: [{ dimensions: {}, sum_30d: 4, sum_12m: 4, days: [{ day: ago(2), value: 4 }], months: [] }] })]} />);
    expect(screen.getByText(copy.changeUp("1.0 GiB", dayWords(ago(9))))).toBeInTheDocument();
    // Both windows start on the record's first day: one sum, said once.
    expect(figures(container)[1]).toBe("4 in the last 10 days");
    expect(screen.queryByText(/in the last 12 months/)).toBeNull();
    expect(screen.getByText(copy.countingBegan(dayWords(ago(9))))).toBeInTheDocument();
  });

  it("a full window: no tick, no faint baseline, no fold, no note", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    expect(container.querySelector("[data-tick]")).toBeNull();
    expect(container.querySelector("[data-pre]")).toBeNull();
    expect(container.querySelector("[data-fold]")).toBeNull();
    expect(screen.queryByText(/Counting began/)).toBeNull();
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });

  it("offline: the days after the last count are blank and say not counted yet, never a zero", () => {
    const m = sessions({ counted_through: ago(3), series: [{ dimensions: { area: "ring" }, sum_30d: 5, sum_12m: 5, days: [{ day: ago(4), value: 5 }], months: [] }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const said = titles(container);
    for (const n of [2, 1, 0]) {
      expect(said).toContain(copy.dayPending(dayWords(ago(n))));
      expect(said).not.toContain(copy.dayValue(dayWords(ago(n)), "0"));
    }
    expect(said).toContain(copy.dayValue(dayWords(ago(3)), "0"));
    expect(bands()[0].querySelectorAll("[data-column]")).toHaveLength(27);
  });
});

describe("the end row", () => {
  it("on a person's card: its step, no link, no Last counted, and what retention keeps", () => {
    const m = sessions({ collected: { at: new Date().toISOString() } });
    render(<UsageCard title={copy.MEMBER_TITLE} windows={windows(10)} metrics={[m]} />);
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByText(/Last counted/)).toBeNull();
    expect(screen.getByText(copy.keptLine(10))).toBeInTheDocument();
  });

  // A-23 part 5 frame 23: a person's card never has the link, so it keeps no empty seat, and its faint
  // facts end at the card's edge rather than 140px short of it.
  it("keeps no empty seat on a person's card; a system's card keeps its seat with or without the link", () => {
    const m = sessions({ began: ago(2) });
    const { container, unmount } = render(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[m]} />);
    expect(container.querySelector("[data-end] [data-action-seat]")).toBeNull();
    unmount();
    const system = render(<UsageCard label={copy.OVERVIEW_TITLE} windows={windows()} metrics={[sessions({ watermark: null })]} target="gym" />);
    expect(system.container.querySelector("[data-end] [data-action-seat]")).not.toBeNull();
  });
});

describe("a person with no account on the system", () => {
  it("has no usage card when nothing of theirs is counted, not a card of empty sections (A-23 part 5 frame 23)", () => {
    const usage = { reported: true as const, metrics: [sessions({ series: [] }), homeBytes({ against: undefined, series: [] })], ...windows() };
    const { container } = render(<ViewUsage view={view(usage, { account: undefined })} person="Rey Mysterio" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("still has it where something of theirs was counted, so it is never nowhere", () => {
    const usage = { reported: true as const, metrics: [sessions(), homeBytes({ against: undefined, series: [] })], ...windows() };
    render(<ViewUsage view={view(usage, { account: undefined })} person="Rey Mysterio" />);
    expect(screen.getByRole("group", { name: copy.operatorTitle("Rey Mysterio") })).toBeInTheDocument();
  });

  it("with an account, the card stays even with nothing counted", () => {
    const usage = { reported: true as const, metrics: [sessions({ series: [] })], ...windows() };
    render(<ViewUsage view={view(usage)} person="Rey Mysterio" />);
    expect(screen.getByText(copy.NOTHING_COUNTED)).toBeInTheDocument();
  });
});

describe("add-on text is quoted, capped and escaped", () => {
  it("caps a long reason and a long label, and never turns markup into elements", () => {
    const long = `<b>x</b>${"y".repeat(400)}`;
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ label: long, gaps: [{ ...GAP, reason: long }] })]} />);
    openFold();
    expect(container.querySelector("b")).toBeNull();
    const line = screen.getByText(/Area: “ring”: “<b>x<\/b>y+…”/);
    expect(line.textContent!.length).toBeLessThan(200);
    expect(screen.getAllByRole("heading").find((h) => h.textContent!.startsWith("<b>"))!.textContent!.length).toBeLessThanOrEqual(60);
  });
});

describe("a counted metric with no series", () => {
  it("says nothing counted, a real zero, not 'not counted'", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [] })]} />);
    expect(screen.getByText(copy.NOTHING_COUNTED)).toBeInTheDocument();
    expect(screen.queryByText(copy.NOT_COUNTED)).toBeNull();
  });

  // Staging, 21 Nov 2026: a share they can open with auditing off has no series of theirs, only a gap.
  it("never says nothing counted when a day in the window was not counted, and gives the reason", () => {
    const off = { ...GAP, start: ago(29), end: ago(-1), reason: "Auditing is off for this share" };
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [], gaps: [off] }), level({ series: [], gaps: [off] })]} />);
    expect(screen.queryByText(copy.NOTHING_COUNTED)).toBeNull();
    expect(screen.queryByText(copy.NO_RECENT_VALUE)).toBeNull();
    expect(screen.getAllByText(copy.NOT_ALL_COUNTED)).toHaveLength(2);
    openFold();
    expect(screen.getByText(/Area: “ring”: “Auditing is off for this share”/)).toBeInTheDocument();
  });

  it("a gap only before the window, or before the record, leaves nothing counted", () => {
    const old = { ...GAP, start: ago(60), end: ago(30) };
    const began = { ...GAP, kind: "began" as const, start: ago(20), end: ago(10) };
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [], gaps: [old, began] })]} />);
    expect(screen.getByText(copy.NOTHING_COUNTED)).toBeInTheDocument();
  });
});

describe("lines left out because they name a person", () => {
  it("says how many beside the lines it shows", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ left_out: 2 })]} />);
    expect(screen.getByText(copy.leftOut(2))).toBeInTheDocument();
    expect(copy.leftOut(1)).toBe("1 more line is not shown here, as the add-on named a person in it.");
  });

  it("never says nothing was counted when every line was left out", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [], left_out: 1 })]} />);
    expect(screen.getByText(copy.leftOut(1))).toBeInTheDocument();
    expect(screen.queryByText(copy.NOTHING_COUNTED)).toBeNull();
  });
});

describe("a person's view of a system", () => {
  const storage = (usage: Parameters<typeof view>[0]) =>
    ({
      ...view(usage, { kind: "storage", entitlement_schema: [{ name: "quota_field", role: "limit", unit: "bytes", label: "Limit" }] as never }),
      storage: { usable: true, needs_password: false, places_readable: true, places: [{ share: "a", path: "/a", access: "change" as const, home: true, usage: { used_bytes: 18.4 * GIB, quota_bytes: 20 * GIB } }] },
    });

  it("titles the member's card as theirs and the operator's by the person's name", () => {
    const usage = { reported: true as const, metrics: [sessions()], ...windows() };
    const { rerender } = render(<ViewUsage view={view(usage)} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    rerender(<ViewUsage view={view(usage)} person="Rey Mysterio" />);
    expect(screen.getByRole("group", { name: "What Rey Mysterio has used" })).toBeInTheDocument();
  });

  it("leaves out the home folder where the kind draws it under its meter (R1), and draws no meter", () => {
    render(<ViewUsage view={storage({ reported: true, metrics: [homeBytes(), sessions()], ...windows() })} />);
    expect(screen.queryByText("Home folder used")).toBeNull();
    expect(screen.getByRole("heading", { name: "Sessions" })).toBeInTheDocument();
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("keeps the home folder on a kind with no meter of its own", () => {
    render(<ViewUsage view={view({ reported: true, metrics: [homeBytes(), sessions()], ...windows() })} />);
    expect(screen.getByRole("heading", { name: "Home folder used" })).toBeInTheDocument();
  });

  it("draws nothing where the add-on does not report, no per-person metric is left, or the view has no kind", () => {
    expect(render(<ViewUsage view={view({ reported: false })} />).container).toBeEmptyDOMElement();
    expect(render(<ViewUsage view={view({ reported: true, metrics: [], ...windows() })} />).container).toBeEmptyDOMElement();
    expect(render(<ViewUsage view={storage({ reported: true, metrics: [homeBytes()], ...windows() })} />).container).toBeEmptyDOMElement();
    expect(render(<ViewUsage view={view({ reported: true, metrics: [sessions()], ...windows() }, { kind: undefined })} />).container).toBeEmptyDOMElement();
  });

  it("says Syndra could not read its own numbers when usage is absent, with its step, never 'no data'", () => {
    render(<ViewUsage view={view(undefined)} />);
    expect(screen.getByText(copy.UNAVAILABLE)).toBeInTheDocument();
    expect(screen.queryByText(/no data/i)).toBeNull();
  });

  it("sits beside the kind's card on the member's page, from the view alone", () => {
    const fetchStub = makeProxyFetch();
    vi.stubGlobal("fetch", fetchStub.fetchImpl);
    render(<MemberTargetCard view={view({ reported: true, metrics: [sessions()], ...windows() })} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    expect(fetchStub.calls).toEqual([]);
  });

  describe("the home folder's month (R1)", () => {
    it("is its change, its band against the limit Syndra read with the limit's word, and its own axis", () => {
      const { container } = render(<HomeMonth view={storage({ reported: true, metrics: [homeBytes()], ...windows() })} />);
      const svg = bands()[0];
      expect(svg.querySelector("[data-limit]")).not.toBeNull();
      expect(within(svg as unknown as HTMLElement).getByText(copy.limitWord("20.0 GiB"))).toBeInTheDocument();
      expect(screen.getByText(/^Up .+ since /)).toBeInTheDocument();
      expect(container.querySelectorAll('[aria-live="polite"]')).toHaveLength(1);
    });

    // The owner's ruling of 21 Nov 2026, "Only that home affected": Charlotte's home is locked for 14
    // days. The add-on's gap is over the totals alone (scope "total"): Booker's chart draws every one of
    // his 14 days, with his change line, under his meter and in the usage card alike; only the
    // system's total hatches. Charlotte's own gap (over her alone, sent plain on her page) hatches hers.
    it("draws a person's own days under a gap over the totals alone; only the total hatches", () => {
      const locked: StatsGap = { start: ago(14), end: ago(0), scope: "total", kind: "unread", reason: "Not counted, could not be read: a home folder in main (locked)" };
      const fortnight = (base: number) => Array.from({ length: 14 }, (_, i) => ({ day: ago(14 - i), value: base + i * GIB }));
      const booker = homeBytes({ gaps: [locked], series: [{ dimensions: {}, latest: { day: ago(1), value: 13 * GIB }, days: fortnight(0) }] });
      const points = (root: Element) => Array.from(root.querySelectorAll("[data-line]")).flatMap((l) => (l.getAttribute("points") ?? "").split(" ").filter(Boolean));

      const month = render(<HomeMonth view={storage({ reported: true, metrics: [booker], ...windows(400) })} />);
      expect(month.container.querySelectorAll("[data-gap]")).toHaveLength(0);
      expect(points(month.container)).toHaveLength(14);
      expect(screen.getByText(/^Up .+ since /)).toBeInTheDocument();
      month.unmount();

      const card = render(<ViewUsage view={view({ reported: true, metrics: [booker], ...windows(400) })} />);
      expect(card.container.querySelectorAll("[data-gap]")).toHaveLength(0);
      expect(points(card.container)).toHaveLength(14);
      expect(screen.queryByText(/a home folder in main/)).toBeNull();
      card.unmount();

      const total = render(<UsageCard label={copy.OVERVIEW_TITLE} windows={windows()} metrics={[homeBytes({ gaps: [locked], series: [{ dimensions: {}, latest: { day: ago(1), value: 40 * GIB }, days: fortnight(27 * GIB) }] })]} target="gym" />);
      expect(total.container.querySelectorAll("[data-gap]")).toHaveLength(1);
      expect(points(total.container)).toHaveLength(0);
      total.unmount();

      const charlotte = homeBytes({ gaps: [{ ...locked, scope: undefined, reason: "Not counted: the home folder is encrypted and locked on TrueNAS" }], series: [{ dimensions: {}, days: [] }] });
      const hers = render(<HomeMonth view={storage({ reported: true, metrics: [charlotte], ...windows(400) })} />);
      expect(hers.container.querySelectorAll("[data-gap]")).toHaveLength(1);
    });

    it("is on its own scale with no limit line where the limit is 0 or was not read", () => {
      const none = storage({ reported: true, metrics: [homeBytes()], ...windows() });
      none.storage.places[0].usage = { used_bytes: 1 } as never;
      const { unmount } = render(<HomeMonth view={none} />);
      expect(bands()[0].querySelector("[data-limit]")).toBeNull();
      unmount();
      render(<HomeMonth view={{ ...none, health: "offline" }} />);
      expect(bands()[0].querySelector("[data-limit]")).toBeNull();
    });

    it("is nothing on a kind with no meter, or before the first count", () => {
      expect(render(<HomeMonth view={view({ reported: true, metrics: [homeBytes()], ...windows() })} />).container).toBeEmptyDOMElement();
      expect(render(<HomeMonth view={storage({ reported: true, metrics: [homeBytes({ watermark: null, series: [] })], ...windows() })} />).container).toBeEmptyDOMElement();
    });

    // Wherever the month cannot draw (not counted yet, no value, several series), the usage card keeps
    // the metric and says so: one answer, `meteredMetric`, so it is never shown nowhere.
    it.each([
      ["not counted yet", { watermark: null, series: [] }],
      ["no value", { series: [] }],
      ["several series", { series: [homeBytes().series[0], { ...homeBytes().series[0], dimensions: { pool: "b" } }] }],
    ] as Array<[string, Partial<StatsMetric>]>)("leaves the home folder in the usage card when the month cannot draw: %s", (_, over) => {
      const v = storage({ reported: true, metrics: [homeBytes(over), sessions()], ...windows() });
      const { container } = render(
        <>
          <HomeMonth view={v} />
          <ViewUsage view={v} />
        </>,
      );
      expect(container.querySelector("[data-home-month]")).toBeNull();
      expect(screen.getAllByRole("region", { name: "Home folder used" })).toHaveLength(1);
    });
  });
});

describe("the target overview", () => {
  const stubStats = (body: unknown) => {
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/gym/stats", () => body);
    vi.stubGlobal("fetch", stub.fetchImpl);
    return stub;
  };

  it("reads the admin endpoint for this target, and draws totals by breakdown with hatched gaps and the unbound line", async () => {
    const stub = stubStats(targetStats());
    const { container } = render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText("61")).toBeInTheDocument();
    expect(figures(container)).toEqual(["61 in the last 30 days"]);
    expect(stub.calls.map((c) => c.url)).toEqual(["/api/proxy/targets/gym/stats"]);
    expect(container.querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(screen.getByText(copy.unbound(1))).toBeInTheDocument();
  });

  it("ends on its step and the one link to Insights, Usage at this system, and how old the count is, once", async () => {
    const t = targetStats();
    if (t.usage.reported) t.usage.metrics.push(level({ name: "b", collected: { at: new Date(Date.now() - 60 * 60_000).toISOString() } }));
    stubStats(t);
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.OVERVIEW_NEXT)).toBeInTheDocument();
    const link = screen.getByRole("link", { name: new RegExp(copy.OVERVIEW_NEXT_LINK) });
    expect(link).toHaveAttribute("href", "/insights?tab=usage#usage-gym");
    expect(link.closest("[data-action-seat]")).toHaveAttribute("data-seat", "narrow");
    expect(link.closest("[data-end]")!.querySelector("[data-action-seat]")).toBe(link.closest("[data-action-seat]"));
    // The newest count's age, said once for the card.
    expect(screen.getAllByText(/Last counted/)).toHaveLength(1);
    expect(screen.getByText(copy.lastCounted(formatRelative(new Date(Date.now() - 4 * 60_000).toISOString())))).toBeInTheDocument();
  });

  it("says what the last count did not keep only in Advanced, inside the fold", async () => {
    stubStats(targetStats());
    const { unmount } = render(withQuery(<TargetStats target="gym" />, true));
    await screen.findByText("61");
    openFold();
    expect(screen.getByText(copy.droppedFor("Sessions", [copy.DROPPED.series(3)]))).toBeInTheDocument();
    unmount();
    render(withQuery(<TargetStats target="gym" />, false));
    await screen.findByText("61");
    openFold();
    expect(screen.queryByText(/the last count did not keep/)).toBeNull();
    expect(screen.getByText(/Auditing is off in this area/)).toBeInTheDocument();
  });

  it("counts accounts in the plural", async () => {
    const t = targetStats();
    if (t.usage.reported) t.usage.metrics[0].collected = { at: null, unbound_accounts: 2 };
    stubStats(t);
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(/2 accounts Syndra does not manage also used it. They are in the totals./)).toBeInTheDocument();
  });

  it("says this add-on does not report statistics when the feature is absent, which is not 'no data'", async () => {
    stubStats(targetStats({ usage: { reported: false } }));
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.ABSENT)).toBeInTheDocument();
    expect(screen.queryByText(/no data/i)).toBeNull();
  });

  it("says Syndra could not read when the read fails: one sentence, no seats", async () => {
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/gym/stats", () => ({ __status: 500, body: { error: "DB_ERROR", message: "x" } }));
    vi.stubGlobal("fetch", stub.fetchImpl);
    const { container } = render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.UNAVAILABLE)).toBeInTheDocument();
    expect(rows(container)).toHaveLength(0);
    expect(screen.queryByText(copy.NOTHING_TO_DO)).toBeNull();
  });

  it("is a loading row while it reads", () => {
    stubStats(new Promise(() => {}));
    render(withQuery(<TargetStats target="gym" />));
    expect(screen.getByLabelText(copy.OVERVIEW_TITLE)).toHaveAttribute("aria-busy", "true");
  });

  // The owner's ruling of 18 Nov 2026: storage says "each share", and only storage does.
  it("words the way to Insights in the system's kind: each share on storage, never share elsewhere", async () => {
    stubStats(targetStats());
    const { unmount, container } = render(withQuery(<TargetStats target="gym" kind="storage" />));
    expect(await screen.findByText(STORAGE_USAGE_STEP)).toBeInTheDocument();
    expect(container.textContent).toMatch(/each share/);
    unmount();
    for (const kind of ["inventory", "machine", undefined]) {
      const { unmount: done, container: c } = render(withQuery(<TargetStats target="gym" kind={kind} />));
      expect(await screen.findByText(copy.OVERVIEW_NEXT)).toBeInTheDocument();
      expect(c.querySelector("[data-end]")!.textContent, String(kind)).not.toMatch(/\bshares?\b/i);
      done();
    }
  });

  it("offers Insights only where there is something there: a system with no per-person metric ends on Nothing for you to do", async () => {
    const t = targetStats();
    if (t.usage.reported) t.usage.metrics = t.usage.metrics.map((m) => ({ ...m, per_subject: false }));
    stubStats(t);
    const { container } = render(withQuery(<TargetStats target="gym" kind="storage" />));
    await screen.findByText("61");
    expect(container.querySelector("[data-end] [data-action-seat]")).toHaveAttribute("data-seat", "narrow"); // empty, and still in its place
    expect(container.querySelector("[data-end]")!.textContent).toMatch(new RegExp(`^${copy.NOTHING_TO_DO}`));
    expect(screen.queryByRole("link", { name: new RegExp(copy.OVERVIEW_NEXT_LINK) })).toBeNull();
    expect(screen.queryByText(STORAGE_USAGE_STEP)).toBeNull();
  });
});

describe("each kind of gap (contract 2: the add-on labels it)", () => {
  const g = (from: number, to: number, kind: StatsGap["kind"], reason: string): StatsGap => ({ start: ago(from), end: ago(to), kind, reason });

  it.each([
    ["unread", "The NAS could not be read"],
    ["dropped", "The NAS no longer keeps its log of these days"],
    ["unobserved", "Auditing does not record everyone on this share"],
  ] as const)("%s: hatched, and listed in the fold with its reason", (kind, reason) => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ gaps: [g(8, 6, kind, reason)] })]} />);
    expect(bands()[0].querySelectorAll("[data-gap]")).toHaveLength(1);
    openFold();
    expect(screen.getByText(new RegExp(reason))).toBeInTheDocument();
  });

  it("began: before the record, never hatched, never listed, never a zero", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ began: ago(6), gaps: [g(96, 6, "began", "Not collected yet: counting began on a day")] })]} />);
    expect(bands()[0].querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(bands()[0].querySelector("[data-tick]")).not.toBeNull();
    expect(container.querySelector("[data-fold]")).toBeNull();
    expect(titles(bands()[0]).filter((t) => t.endsWith("before counting began"))).toHaveLength(23);
    expect(bands()[0].querySelectorAll("[data-column]")).toHaveLength(7);
  });

  it("a year-old system with a later outage: the outage is an unread gap, hatched and listed, and no tick moves to it", () => {
    // The began gap ended a year ago and is not in the 12 months read; the backend still names the day.
    const m = level({ began: ago(365), gaps: [g(40, 20, "unread", "The NAS could not be read")], series: [{ dimensions: {}, latest: { day: ago(2), value: 5 * GIB }, days: [{ day: ago(3), value: 4 * GIB }, { day: ago(2), value: 5 * GIB }] }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const svg = bands()[0];
    expect(svg.querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(svg.querySelector("[data-tick]")).toBeNull();
    expect(svg.querySelector("[data-pre]")).toBeNull();
    expect(titles(svg).filter((t) => t.endsWith("before counting began"))).toHaveLength(0);
    expect(screen.queryByText(/Counting began/)).toBeNull();
    expect(screen.getByText(copy.changeUp("1.0 GiB", dayWords(ago(3))))).toBeInTheDocument();
    openFold();
    expect(container.textContent).toMatch(/The NAS could not be read/);
  });

  it("a short buffer (a week): the days before counting began are before the record, never a counter's zero", () => {
    const m = sessions({ began: ago(6), gaps: [g(13, 6, "began", "Not collected yet")], series: [{ dimensions: { area: "ring" }, sum_30d: 2, sum_12m: 2, days: [{ day: ago(3), value: 2 }], months: [] }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const said = titles(bands()[0]);
    expect(said.filter((t) => t.endsWith("before counting began"))).toHaveLength(23);
    expect(said.filter((t) => t.endsWith(": 0"))).toHaveLength(6);
    expect(figures(container)).toEqual(["2 in the last 7 days"]);
  });

  it("a new member's person card: no began gap of theirs, so their quiet days are real zeros and nothing reads as before counting", () => {
    // A person's view of a system counting for a year; the member's own points begin last week.
    const m = sessions({ began: ago(300), series: [{ dimensions: { area: "ring" }, sum_30d: 3, sum_12m: 3, days: [{ day: ago(4), value: 3 }], months: [{ month: ago(4).slice(0, 7), value: 3 }] }] });
    const { container } = render(<UsageCard title={copy.operatorTitle("Becky Lynch")} windows={windows(400)} metrics={[m]} />);
    const svg = bands()[0];
    expect(svg.querySelector("[data-tick]")).toBeNull();
    expect(titles(svg).filter((t) => t.endsWith("before counting began"))).toHaveLength(0);
    expect(svg.querySelectorAll("[data-column]")).toHaveLength(30);
    expect(figures(container)).toEqual(["3 in the last 30 days"]);
  });
});

describe("the band's fidelity", () => {
  it("draws the read day behind the band on the inner radius, not as a pill", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    fireEvent.pointerEnter(bands()[0].querySelectorAll("rect:not([data-column])")[28]);
    const svg = bands()[0];
    const read = svg.parentElement!.querySelector("[data-read]")!;
    expect(read.tagName).toBe("DIV");
    expect(read).toHaveClass("rounded-inner");
    // Behind the marks: the tint comes first and the band is positioned after it, so the band paints over it.
    expect(read.compareDocumentPosition(svg) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(svg).toHaveClass("relative");
    // And it never takes the pointer, which would end the read it shows (the readout looping).
    expect(read).toHaveClass("pointer-events-none");
  });

  it("keeps the limit's word off the line on a folder 95% full, and moves it left of a point over the limit", () => {
    const storage = (used: number) =>
      ({
        ...view({ reported: true, metrics: [homeBytes({ series: [{ dimensions: {}, latest: { day: ago(1), value: used }, days: Array.from({ length: 29 }, (_, i) => ({ day: ago(29 - i), value: i >= 25 ? used : Math.min(used, 19 * GIB) })) }] })], ...windows() }, { kind: "storage", entitlement_schema: [{ name: "quota_field", role: "limit", unit: "bytes", label: "Limit" }] as never }),
        storage: { usable: true, needs_password: false, places_readable: true, places: [{ share: "a", path: "/a", access: "change" as const, home: true, usage: { used_bytes: used, quota_bytes: 20 * GIB } }] },
      });
    const { unmount } = render(<HomeMonth view={storage(19 * GIB)} />);
    let svg = bands()[0];
    const word = svg.querySelector("[data-limit-word]")!;
    const lineY = Number(svg.querySelector("[data-limit]")!.getAttribute("y1"));
    // The word's baseline is above the limit line, and every point of the line is at or below it (SVG y grows down).
    expect(Number(word.getAttribute("y"))).toBeLessThan(lineY - 1);
    const ys = svg.querySelector("[data-line]")!.getAttribute("points")!.split(" ").map((p) => Number(p.split(",")[1]));
    expect(Math.min(...ys)).toBeGreaterThanOrEqual(lineY - 1);
    expect(word.getAttribute("text-anchor")).toBe("end");
    unmount();
    render(<HomeMonth view={storage(22 * GIB)} />);
    svg = bands()[0];
    expect(svg.querySelector("[data-limit-word]")!.getAttribute("text-anchor")).toBe("start");
  });

  it("draws no limit word where a point over the limit would lie under it at both ends", () => {
    const over = 22 * GIB;
    const v = {
      ...view({ reported: true, metrics: [homeBytes({ series: [{ dimensions: {}, latest: { day: ago(0), value: over }, days: Array.from({ length: 30 }, (_, i) => ({ day: ago(29 - i), value: over })) }] })], ...windows() }, { kind: "storage", entitlement_schema: [{ name: "quota_field", role: "limit", unit: "bytes", label: "Limit" }] as never }),
      storage: { usable: true, needs_password: false, places_readable: true, places: [{ share: "a", path: "/a", access: "change" as const, home: true, usage: { used_bytes: over, quota_bytes: 20 * GIB } }] },
    };
    render(<HomeMonth view={v} />);
    expect(bands()[0].querySelector("[data-limit]")).not.toBeNull();
    expect(bands()[0].querySelector("[data-limit-word]")).toBeNull();
  });
});

describe("a gap the add-on sent is never lost", () => {
  it("one with no reason is hatched like any other, never a zero, and the fold says no reason was given", () => {
    const silent: StatsGap = { start: ago(8), end: ago(6), dimensions: { area: "ring" }, kind: "unread", reason: "" };
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ gaps: [silent] })]} />);
    const ring = bands()[0];
    expect(ring.querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(titles(ring)).not.toContain(copy.dayValue(dayWords(ago(7)), "0"));
    openFold();
    expect(container.textContent).toContain(copy.gapLine(`${dayWords(ago(8)).split(" ")[0]}–${dayWords(ago(7))}`, "Area: “ring”", copy.NO_REASON).split(", ")[1]);
  });
});

describe("what Syndra keeps", () => {
  it("says a 12-month sum is of the last 90 days when only 90 are kept, draws nothing before the record, and says what is kept", () => {
    const { container } = render(<UsageCard title="t" windows={windows(90)} metrics={[sessions()]} />);
    expect(figures(container)[0]).toBe("7 in the last 30 days");
    expect(screen.getByText("37 in the last 90 days")).toBeInTheDocument();
    expect(container.querySelector("[data-pre]")).toBeNull();
    expect(screen.getByText(copy.keptLine(90))).toBeInTheDocument();
  });

  it("with 400 kept, or on a totals page, the year is the ordinary one", () => {
    const { rerender } = render(<UsageCard title="t" windows={windows(400)} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
  });

  it("with 10 kept, 10 days are the record and the 20 before are before it: not hatched, not a zero, said as kept", () => {
    const { container } = render(<UsageCard title="t" windows={windows(10)} metrics={[sessions()]} />);
    expect(figures(container)[0]).toBe("7 in the last 10 days");
    expect(screen.queryByText(/in the last 12 months|in the last 30 days/)).toBeNull();
    const ring = bands()[0];
    expect(ring.querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(ring.querySelector("[data-pre]")).not.toBeNull();
    expect(ring.querySelector("[data-tick]")).not.toBeNull();
    const said = titles(ring);
    expect(said.filter((t) => t.includes("kept for only"))).toHaveLength(20);
    expect(said).toContain(copy.dayValue(dayWords(ago(9)), "0"));
    expect(screen.getByText(copy.keptLine(10))).toBeInTheDocument();
  });

  it("counts n to the backend's day, so a response cached across midnight still says the right n", () => {
    const w = { ...windows(10), today: ago(1), kept_from: ago(10), window_30d_from: ago(10), window_12m_from: ago(10) };
    const { container } = render(<UsageCard title="t" windows={w} metrics={[sessions()]} />);
    expect(figures(container)[0]).toBe("7 in the last 10 days");
  });

  it("names the 12-month sum from the backend's start", () => {
    const { rerender } = render(<UsageCard title="t" windows={{ today: windows().today, kept_from: ago(199), window_30d_from: ago(29), window_12m_from: ago(199) }} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 200 days")).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={{ today: windows().today, kept_from: ago(399), window_30d_from: ago(29), window_12m_from: ago(340) }} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
  });

  it("does not say what is kept on a metric not counted yet", () => {
    render(<UsageCard title="t" windows={windows(10)} metrics={[sessions({ watermark: null, series: [] })]} />);
    expect(screen.queryByText(copy.keptLine(10))).toBeNull();
  });
});

describe("a value with no neighbour is a dot", () => {
  it("never joins a level over a day with no sample: only adjacent sampled days are a line, the rest are dots", () => {
    const days = [6, 5, 2, 0];
    const m = level({ series: [{ dimensions: {}, latest: { day: ago(0), value: 4 }, days: days.map((d) => ({ day: ago(d), value: 4 })) }] });
    render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const svg = bands()[0];
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(1);
    expect(svg.querySelectorAll("[data-line]")[0].getAttribute("points")!.split(" ")).toHaveLength(2);
    expect(svg.querySelectorAll("[data-area]")).toHaveLength(1);
    const dots = svg.querySelectorAll("[data-dot]");
    expect(dots).toHaveLength(2);
    for (const d of Array.from(dots)) {
      expect(d).toHaveAttribute("r", "2.5");
      expect(d).toHaveAttribute("stroke", "var(--surface-1)");
    }
  });
});

describe("motion (A-23 §7)", () => {
  const with4 = (more: boolean) => sessions({ series: [{ dimensions: { area: "ring" }, sum_30d: 3, sum_12m: 3, days: [{ day: ago(2), value: 3 }, ...(more ? [{ day: ago(1), value: 2 }] : [])], months: [] }], counted_through: more ? ago(1) : ago(2) });

  it("draws everything complete on the first paint, and tints in only a day that gained a value on a later read", () => {
    motionInTests(true);
    const { rerender } = render(<UsageCard title="t" windows={windows()} metrics={[with4(false)]} />);
    expect(bands()[0].querySelectorAll(".tint-in")).toHaveLength(0);
    rerender(<UsageCard title="t" windows={windows()} metrics={[with4(true)]} />);
    const fresh = bands()[0].querySelectorAll(".tint-in");
    expect(fresh).toHaveLength(1);
    expect(fresh[0].hasAttribute("data-column")).toBe(true);
  });

  it("under reduced motion, a later read changes at once", () => {
    const { rerender } = render(<UsageCard title="t" windows={windows()} metrics={[with4(false)]} />);
    rerender(<UsageCard title="t" windows={windows()} metrics={[with4(true)]} />);
    expect(bands()[0].querySelectorAll(".tint-in")).toHaveLength(0);
  });
});

describe("what the last count did not keep, in words", () => {
  it("says one reason in the singular and in the plural, and never a name", () => {
    expect(copy.DROPPED.not_a_day(1)).toBe("1 value that was not for a whole day");
    expect(copy.DROPPED.not_a_day(3)).toBe("3 values that were not for a whole day");
    expect(copy.DROPPED.bad_value(2)).toBe("2 values or gaps that were empty or below zero");
    expect(copy.droppedFor("Sessions", [copy.DROPPED.not_a_day(1)])).toBe("Sessions: the last count did not keep 1 value that was not for a whole day.");
  });

  it("has words for every reason the backend sends", () => {
    for (const reason of ["not_a_day", "bad_value", "not_per_subject", "unbound_subject", "undeclared_dimension", "past_retention", "series"]) {
      expect(copy.DROPPED[reason](2), reason).toMatch(/^2 /);
    }
  });
});

describe("the storage kind's limit", () => {
  const field = (name: string, unit: string) => ({ name, role: "limit", unit, label: name });
  const read = (schema: unknown[], over = {}) => {
    const v = view({ reported: true, metrics: [homeBytes()], ...windows() }, { kind: "storage", entitlement_schema: schema as never, ...over });
    return { ...v, storage: { usable: true, needs_password: false, places: [{ share: "a", path: "/a", access: "change" as const, home: true, usage: { used_bytes: 1, quota_bytes: 20 * GIB } }] } };
  };

  it("takes the one limit in bytes, and none when two are (it would be a guess) or none is", async () => {
    const { storageLimits } = await import("@/components/kinds/storage/limits");
    expect(storageLimits(read([field("quota_field", "bytes")]))).toEqual({ quota_field: 20 * GIB });
    expect(storageLimits(read([field("quota_field", "bytes"), field("roles_limit", "bytes")]))).toEqual({});
    expect(storageLimits(read([field("quota_field", "bytes"), field("seats", "items")]))).toEqual({ quota_field: 20 * GIB });
    expect(storageLimits(read([field("seats", "items")]))).toEqual({});
  });

  it("is not read while offline or where the person cannot change the place", async () => {
    const { storageLimits } = await import("@/components/kinds/storage/limits");
    const schema = [field("quota_field", "bytes")];
    expect(storageLimits(read(schema, { health: "offline" }))).toEqual({});
    const readOnly = read(schema);
    readOnly.storage.places[0] = { ...readOnly.storage.places[0], access: "read" as never };
    expect(storageLimits(readOnly)).toEqual({});
  });
});

// A-24 §10: on a system's overview a per-person figure opens the people behind it in Insights' Usage.
describe("a figure opens its list", () => {
  it("links a per-person metric's figures, and leaves one with no list behind it plain", () => {
    render(
      withQuery(
        <UsageCard label="Usage" windows={windows()} metrics={[sessions(), sessions({ name: "volume.total", label: "Total", per_subject: false })]} target="nas" />,
        true,
      ),
    );
    const links = screen.getAllByRole("link", { name: /open Usage in Insights/ });
    expect(links).toHaveLength(2); // the per-person metric's two series; the total's stay plain
    for (const link of links) expect(link).toHaveAttribute("href", insightsUsageHref("nas"));
    expect(links[0]).toHaveAccessibleName("7 in the last 30 days: open Usage in Insights");
  });

  it("has no link on a person's card, which names no system", () => {
    render(withQuery(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[sessions()]} />));
    expect(screen.queryByRole("link", { name: /open Usage in Insights/ })).toBeNull();
  });
});

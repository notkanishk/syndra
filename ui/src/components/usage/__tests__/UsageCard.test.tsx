// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import * as copy from "@/components/usage/copy";
import { TargetStats } from "@/components/usage/TargetStats";
import { UsageCard, ViewUsage } from "@/components/usage/UsageCard";
import { dayWords } from "@/lib/usage";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { ago, GAP, GIB, homeBytes, sessions, targetStats, view, windows } from "./fixtures";

const withQuery = (ui: ReactElement) => <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{ui}</QueryClientProvider>;
afterEach(() => vi.unstubAllGlobals());

const titles = (root: HTMLElement) => Array.from(root.querySelectorAll("svg title")).map((t) => t.textContent ?? "");

describe("What I have used: data", () => {
  it("sums a counter over both windows, one row per series, and shows a gauge's latest with its day", () => {
    render(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[homeBytes(), sessions()]} limits={{ quota_field: 20 * GIB }} />);
    expect(screen.getByRole("heading", { name: "Home folder used" })).toBeInTheDocument();
    expect(screen.getByText("7 in the last 30 days")).toBeInTheDocument();
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
    // A series with nothing in the window is its own honest zero; series are never added together.
    expect(screen.getByText("0 in the last 30 days")).toBeInTheDocument();
    expect(screen.getByText("Area: “ring”")).toBeInTheDocument();
    expect(screen.getByText("Area: “mat”")).toBeInTheDocument();
    expect(screen.getByText(/^18\.4 GiB on \d+ \w+/)).toBeInTheDocument();
  });

  it("draws the gauge as a meter over the OBSERVED limit", () => {
    render(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[homeBytes()]} limits={{ quota_field: 40 * GIB }} />);
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuemax", String(40 * GIB));
    expect(meter).toHaveAttribute("aria-valuenow", String(18.4 * GIB));
    expect(screen.getByText("18.4 GiB of 40.0 GiB")).toBeInTheDocument();
    expect(meter.firstElementChild).toHaveClass("bg-accent");
  });

  it("turns the meter warn at the limit (see Meter.test for the threshold), says no limit for 0, and says it was not read for none", () => {
    const { rerender } = render(<UsageCard title="t" windows={windows()} metrics={[homeBytes()]} limits={{ quota_field: 18 * GIB }} />);
    expect(screen.getByRole("meter").firstElementChild).toHaveClass("bg-warn");
    rerender(<UsageCard title="t" windows={windows()} metrics={[homeBytes()]} limits={{ quota_field: 0 }} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.getByText(copy.NO_LIMIT)).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={windows()} metrics={[homeBytes()]} limits={{}} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.getByText(copy.LIMIT_UNREAD)).toBeInTheDocument();
    // The value is still said, so a missing limit never hides the number.
    expect(screen.getByText(/^18\.4 GiB on/)).toBeInTheDocument();
  });

  it("draws no meter on a totals page, where no limit belongs to anyone", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[homeBytes()]} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.queryByText(copy.LIMIT_UNREAD)).toBeNull();
  });

  it("draws no meter for a gauge without `against`", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[homeBytes({ against: undefined })]} limits={{ quota_field: 20 * GIB }} />);
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("ends on a next step", () => {
    render(<UsageCard title={copy.MEMBER_TITLE} windows={windows()} metrics={[sessions()]} />);
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });
});

describe("What I have used: gaps", () => {
  const gapped = [sessions({ gaps: [GAP] })];

  it("footnotes the days and the add-on's reason, quoted, and says a blank is not a zero", () => {
    render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    expect(screen.getByText(copy.GAPS_HEADING)).toBeInTheDocument();
    expect(screen.getByText(/Area: “ring”: “Auditing is off in this area”/)).toBeInTheDocument();
    expect(screen.getByText(copy.GAPS_NOTE)).toBeInTheDocument();
  });

  it("hatches the days and breaks the line: no gap day is a zero, and no gap day has a value", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    const rows = screen.getAllByRole("img");
    const ring = rows[0];
    expect(ring.querySelectorAll("[data-gap]")).toHaveLength(1);
    // The line is two runs, one each side of the gap.
    expect(ring.querySelectorAll("[data-line]")).toHaveLength(2);
    const said = titles(container).filter((t) => t.endsWith("couldn't count"));
    expect(said).toHaveLength(3);
    // The other series is not named by the gap: not hatched, one unbroken line.
    expect(rows[1].querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(rows[1].querySelectorAll("[data-line]")).toHaveLength(1);
    // And a day with a value is never said as a zero inside the gap's days.
    const ringTitles = Array.from(ring.querySelectorAll("title")).map((t) => t.textContent);
    for (const day of [ago(8), ago(7), ago(6)]) expect(ringTitles, day).toContain(copy.dayGap(dayWords(day)));
    // The day the gap ends on has data again.
    expect(ringTitles).not.toContain(copy.dayGap(dayWords(ago(5))));
    expect(ringTitles.filter((t) => t?.endsWith(": 0"))).not.toHaveLength(0);
  });

  it("describes the hatching to a screen reader", () => {
    render(<UsageCard title="t" windows={windows()} metrics={gapped} />);
    expect(screen.getAllByRole("img")[0]).toHaveAccessibleName(/Days that could not be counted are hatched/);
    expect(screen.getAllByRole("img")[1]).not.toHaveAccessibleName(/hatched/);
  });

  it("a gauge's gap breaks its line, and a gauge's day with no sample breaks it too", () => {
    const gauge = homeBytes({ against: undefined, gaps: [{ start: ago(8), end: ago(5), reason: "The NAS was off" }], series: [{ dimensions: {}, latest: { day: ago(1), value: 5 }, days: [{ day: ago(12), value: 4 }, { day: ago(10), value: 4 }, { day: ago(3), value: 5 }, { day: ago(1), value: 5 }] }] });
    render(<UsageCard title="t" windows={windows()} metrics={[gauge]} />);
    const svg = screen.getByRole("img");
    // Gauges are never interpolated: days 12 and 10 are not adjacent, so none is joined; each stands alone as a dot.
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(0);
    expect(svg.querySelectorAll("[data-dot]")).toHaveLength(4);
    expect(svg.querySelectorAll("[data-gap]")).toHaveLength(1);
  });
});

describe("add-on text is quoted, capped and escaped", () => {
  it("caps a long reason and a long label, and never turns markup into elements", () => {
    const long = `<b>x</b>${"y".repeat(400)}`;
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ label: long, gaps: [{ ...GAP, reason: long }] })]} />);
    expect(container.querySelector("b")).toBeNull();
    const line = screen.getByText(/Area: “ring”: “<b>x<\/b>y+…”/);
    expect(line.textContent!.length).toBeLessThan(200);
    expect(screen.getByRole("heading").textContent!.length).toBeLessThanOrEqual(60);
  });
});

describe("What I have used: not read yet", () => {
  it("says not counted yet, draws nothing, and its step is what waits", () => {
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[sessions({ watermark: null, series: [] })]} />);
    expect(screen.getByText(copy.NOT_COUNTED)).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeNull();
    expect(screen.getByText(copy.NOT_COUNTED_NEXT)).toBeInTheDocument();
    expect(screen.queryByText(copy.NOTHING_TO_DO)).toBeNull();
  });

  it("says nothing counted for a counted metric with no series: a real zero, not 'not counted'", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[sessions({ series: [] })]} />);
    expect(screen.getByText(copy.NOTHING_COUNTED)).toBeInTheDocument();
    expect(screen.queryByText(copy.NOT_COUNTED)).toBeNull();
  });
});

describe("a person's view of a system", () => {
  it("titles the member's card as theirs and the operator's by the person's name", () => {
    const usage = { reported: true as const, metrics: [sessions()], ...windows() };
    const { rerender } = render(<ViewUsage view={view(usage)} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    rerender(<ViewUsage view={view(usage)} person="Rey Mysterio" />);
    expect(screen.getByRole("group", { name: "What Rey Mysterio has used" })).toBeInTheDocument();
  });

  it("draws nothing where the add-on does not report statistics, or this person has no per-person metric", () => {
    const a = render(<ViewUsage view={view({ reported: false })} />);
    expect(a.container).toBeEmptyDOMElement();
    const b = render(<ViewUsage view={view({ reported: true, metrics: [], ...windows() })} />);
    expect(b.container).toBeEmptyDOMElement();
    const c = render(<ViewUsage view={view({ reported: true, metrics: [sessions()], ...windows() }, { kind: undefined })} />);
    expect(c.container).toBeEmptyDOMElement();
  });

  it("says Syndra could not read its own numbers when usage is absent, with its step, never 'no data'", () => {
    render(<ViewUsage view={view(undefined)} />);
    expect(screen.getByText(copy.UNAVAILABLE)).toBeInTheDocument();
    expect(copy.UNAVAILABLE).toMatch(/Reload this page/);
    expect(screen.queryByText(/no data/i)).toBeNull();
  });

  it("sits beside the kind's card on the member's page, from the view alone", () => {
    const fetchStub = makeProxyFetch();
    vi.stubGlobal("fetch", fetchStub.fetchImpl);
    render(<MemberTargetCard view={view({ reported: true, metrics: [sessions()], ...windows() })} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    expect(screen.getByText("rey_mysterio")).toBeInTheDocument();
    // The card reads what the member's own view carried: it asks for nothing, so it cannot ask for anyone else's.
    expect(fetchStub.calls).toEqual([]);
  });

  it("meters a storage member's gauge over the home limit the view observed, found by the schema's limit field", async () => {
    const { storageLimits } = await import("@/components/kinds/storage/limits");
    const schema = [{ name: "quota_field", role: "limit", unit: "bytes", label: "Limit" }] as never;
    const storage = view({ reported: true, metrics: [homeBytes()], ...windows() }, { kind: "storage", entitlement_schema: schema });
    expect(storageLimits(storage)).toEqual({});
    const read = { ...storage, storage: { usable: true, needs_password: false, places: [{ share: "a", path: "/a", access: "change" as const, home: true, usage: { used_bytes: 1, quota_bytes: 20 * GIB } }] } };
    expect(storageLimits(read)).toEqual({ quota_field: 20 * GIB });
    // No quota on a read place is "no limit" (0), not "not read".
    const unlimited = { ...read, storage: { ...read.storage, places: [{ ...read.storage.places[0], usage: { used_bytes: 1 } }] } };
    expect(storageLimits(unlimited)).toEqual({ quota_field: 0 });
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
    expect(await screen.findByText("61 in the last 30 days")).toBeInTheDocument();
    expect(stub.calls.map((c) => c.url)).toEqual(["/api/proxy/targets/gym/stats"]);
    expect(container.querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(screen.getByText(copy.unbound(1))).toBeInTheDocument();
    expect(screen.getByText(/Last counted 4 min ago/)).toBeInTheDocument();
    expect(screen.getByText(copy.droppedLine([copy.DROPPED.series(3)]))).toBeInTheDocument();
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });

  it("counts accounts in the plural", async () => {
    const t = targetStats();
    if (t.usage.reported) t.usage.metrics[0].collected = { at: null, unbound_accounts: 2 };
    stubStats(t);
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(/2 accounts Syndra does not manage also used it. They are in the totals./)).toBeInTheDocument();
    expect(screen.getByText(copy.NOT_COUNTED)).toBeInTheDocument();
  });

  it("says this add-on does not report statistics when the feature is absent, which is not 'no data'", async () => {
    stubStats(targetStats({ usage: { reported: false } }));
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.ABSENT)).toBeInTheDocument();
    expect(screen.queryByText(/no data/i)).toBeNull();
  });

  it("says Syndra could not read when the read fails, and does not draw an empty card as if it were data", async () => {
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/gym/stats", () => ({ __status: 500, body: { error: "DB_ERROR", message: "x" } }));
    vi.stubGlobal("fetch", stub.fetchImpl);
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.UNAVAILABLE)).toBeInTheDocument();
    expect(within(document.body).queryByText(copy.NOTHING_TO_DO)).toBeNull();
  });
});

describe("what Syndra keeps, and what is not counted yet", () => {
  const ring = (el: HTMLElement) => el.querySelectorAll("svg")[0];
  const said = (svg: Element) => Array.from(svg.querySelectorAll("title")).map((t) => t.textContent);

  it("says a 12-month sum is of the last 90 days when only 90 are kept, and hatches nothing in a 30-day window", () => {
    const { container } = render(<UsageCard title="t" windows={windows(90)} metrics={[sessions()]} />);
    expect(screen.getByText("7 in the last 30 days")).toBeInTheDocument();
    expect(screen.getByText("37 in the last 90 days")).toBeInTheDocument();
    expect(screen.queryByText(/12 months/)).toBeNull();
    expect(ring(container).querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(screen.queryByText(copy.GAPS_HEADING)).toBeNull();
  });

  it("with 400 kept, or on a totals page, the headline and the year are the ordinary ones", () => {
    const { rerender } = render(<UsageCard title="t" windows={windows(400)} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={windows()} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
  });

  it("with exactly 30 kept, the window is whole: the last 30 days, nothing hatched, no footnote, and the year not said twice", () => {
    const { container } = render(<UsageCard title="t" windows={windows(30)} metrics={[sessions()]} />);
    expect(screen.getByText("7 in the last 30 days")).toBeInTheDocument();
    expect(screen.queryByText(/in the last (12 months|\d+ days)$/, { selector: ".text-faint" })).toBeNull();
    expect(ring(container).querySelectorAll("[data-gap]")).toHaveLength(0);
    expect(said(ring(container)).filter((t) => t?.includes("kept for only"))).toHaveLength(0);
    expect(screen.queryByText(copy.keptLine(30))).toBeNull();
    expect(screen.queryByText(copy.GAPS_HEADING)).toBeNull();
  });

  it("with 10 kept, the headline names 10 days, exactly 10 days are drawn unhatched, the 20 before are Syndra's reason, and none is a zero", () => {
    const { container } = render(<UsageCard title="t" windows={windows(10)} metrics={[sessions()]} />);
    expect(screen.getByText("7 in the last 10 days")).toBeInTheDocument();
    expect(screen.queryByText(/in the last 12 months|in the last 30 days/)).toBeNull();
    expect(ring(container).querySelectorAll("[data-gap]")).toHaveLength(1);
    expect(screen.getByText(copy.keptLine(10))).toBeInTheDocument();
    const titles = said(ring(container));
    expect(titles.filter((t) => !t?.includes("kept for only"))).toHaveLength(10);
    expect(titles.filter((t) => t === copy.dayKept(dayWords(ago(10)), 10))).toHaveLength(1);
    expect(titles.filter((t) => t?.includes("kept for only"))).toHaveLength(20);
    expect(titles).toContain(copy.dayValue(dayWords(ago(9)), "0"));
    expect(titles.filter((t) => t?.endsWith(": 0")).length).toBeLessThanOrEqual(10);
    expect(screen.getByText(copy.GAPS_NOTE)).toBeInTheDocument();
  });

  it("names the 12-month sum from the backend's start: cut short by retention it is the last n days, else the 12 months", () => {
    // 300 days always start after the first of the month 11 back (at most 365 days ago, at least 334).
    const { rerender } = render(<UsageCard title="t" windows={windows(300)} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 300 days")).toBeInTheDocument();
    // 350 days reach past it on some days of the month and not on others: the words follow the start the backend read.
    const w = windows(350);
    rerender(<UsageCard title="t" windows={w} metrics={[sessions()]} />);
    expect(screen.getByText(w.window_12m_from === w.kept_from ? "37 in the last 350 days" : "37 in the last 12 months")).toBeInTheDocument();
    // A start the backend says is retention's is said as days from it, whatever the window would have been.
    rerender(<UsageCard title="t" windows={{ kept_from: ago(199), window_30d_from: ago(29), window_12m_from: ago(199) }} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 200 days")).toBeInTheDocument();
    rerender(<UsageCard title="t" windows={{ kept_from: ago(399), window_30d_from: ago(29), window_12m_from: ago(340) }} metrics={[sessions()]} />);
    expect(screen.getByText("37 in the last 12 months")).toBeInTheDocument();
  });

  it("explains the days Syndra does not keep only where days are drawn: not on a metric not counted yet", () => {
    render(<UsageCard title="t" windows={windows(10)} metrics={[sessions({ watermark: null, series: [] })]} />);
    expect(screen.queryByText(copy.keptLine(10))).toBeNull();
    expect(screen.queryByText(copy.GAPS_HEADING)).toBeNull();
  });

  it("reaches a person's card from the view's usage", () => {
    render(<ViewUsage view={view({ reported: true, metrics: [sessions()], ...windows(90) })} />);
    expect(screen.getByText("37 in the last 90 days")).toBeInTheDocument();
  });

  it("never draws a day after the last counted one as a zero: it is blank and says not counted yet", () => {
    const m = sessions({ counted_through: ago(2), series: [{ dimensions: { area: "ring" }, sum_30d: 5, sum_12m: 5, days: [{ day: ago(3), value: 5 }], months: [] }] });
    const { container } = render(<UsageCard title="t" windows={windows()} metrics={[m]} />);
    const titles = said(ring(container));
    expect(titles).toContain(copy.dayPending(dayWords(ago(1))));
    expect(titles).toContain(copy.dayPending(dayWords(ago(0))));
    expect(titles).toContain(copy.dayValue(dayWords(ago(2)), "0"));
    expect(titles).not.toContain(copy.dayValue(dayWords(ago(1)), "0"));
    expect(titles).not.toContain(copy.dayValue(dayWords(ago(0)), "0"));
  });
});

describe("a value with no neighbour is a dot", () => {
  const solo = (instrument: "counter" | "gauge", days: number[], gaps: Array<[number, number]> = []) => {
    const base = instrument === "counter" ? sessions() : homeBytes({ against: undefined });
    return {
      ...base,
      dimensions: undefined,
      gaps: gaps.map(([a, b]) => ({ start: ago(a), end: ago(b), reason: "The NAS was off" })),
      series: [{ dimensions: {}, latest: { day: ago(days[days.length - 1]), value: 4 }, days: days.map((d) => ({ day: ago(d), value: 4 })), sum_30d: 4, sum_12m: 4, months: [] }],
    };
  };

  it("draws a value standing alone between gaps as a dot (r 2.5, accent, a surface ring)", () => {
    // ago(4) and ago(3) are a gap, and so is ago(1): ago(2) stands alone.
    render(<UsageCard title="t" windows={windows()} metrics={[solo("counter", [10, 2], [[4, 2], [1, 0]])]} />);
    const svg = screen.getByRole("img");
    const dots = svg.querySelectorAll("[data-dot]");
    // The lone day and the last day (today, a real zero after the gap) are both alone.
    expect(dots).toHaveLength(2);
    for (const d of Array.from(dots)) {
      expect(d).toHaveAttribute("r", "2.5");
      expect(d).toHaveAttribute("fill", "var(--accent)");
      expect(d).toHaveAttribute("stroke", "var(--surface-1)");
    }
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(1);
  });

  it("never joins a gauge over a day with no sample: only adjacent sampled days are a line, the rest are dots", () => {
    render(<UsageCard title="t" windows={windows()} metrics={[solo("gauge", [6, 5, 2, 0])]} />);
    const svg = screen.getByRole("img");
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(1);
    expect(svg.querySelectorAll("[data-line]")[0].getAttribute("points")!.split(" ")).toHaveLength(2);
    expect(svg.querySelectorAll("[data-dot]")).toHaveLength(2);
  });
});

describe("what the last count did not keep, in words", () => {
  it("says one reason in the singular and in the plural, and never a name", () => {
    expect(copy.DROPPED.not_a_day(1)).toBe("1 value that was not for a whole day");
    expect(copy.DROPPED.not_a_day(3)).toBe("3 values that were not for a whole day");
    expect(copy.DROPPED.bad_value(1)).toBe("1 value or gap that was empty or below zero");
    expect(copy.DROPPED.bad_value(2)).toBe("2 values or gaps that were empty or below zero");
    expect(copy.DROPPED.series(1)).toBe("1 value from more series than Syndra keeps");
    expect(copy.droppedLine([copy.DROPPED.not_a_day(1)])).toBe("The last count did not keep 1 value that was not for a whole day.");
  });

  it("has words for every reason the backend sends, none of them a person's name", () => {
    const sent = ["not_a_day", "bad_value", "not_per_subject", "unbound_subject", "undeclared_dimension", "person_in_dimension", "past_retention", "series"];
    for (const reason of sent) {
      expect(copy.DROPPED[reason], reason).toBeTypeOf("function");
      expect(copy.DROPPED[reason](2)).toMatch(/^2 /);
    }
  });

  it("joins several reasons into one sentence that ends in 'and'", async () => {
    const t = targetStats();
    if (t.usage.reported) t.usage.metrics[0].collected = { at: null, dropped: { not_a_day: 1, past_retention: 4, series: 2, person_in_dimension: 0, mystery: 9 } };
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/gym/stats", () => t);
    vi.stubGlobal("fetch", stub.fetchImpl);
    render(withQuery(<TargetStats target="gym" />));
    expect(
      await screen.findByText("The last count did not keep 1 value that was not for a whole day, 4 values older than Syndra keeps and 2 values from more series than Syndra keeps."),
    ).toBeInTheDocument();
  });
});

describe("the storage kind's limit for a meter", () => {
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

  it("draws nothing while offline or where the person cannot change the place, as the storage card does", async () => {
    const { storageLimits } = await import("@/components/kinds/storage/limits");
    const schema = [field("quota_field", "bytes")];
    expect(storageLimits(read(schema, { health: "offline" }))).toEqual({});
    const readOnly = read(schema);
    readOnly.storage.places[0] = { ...readOnly.storage.places[0], access: "read" as never };
    expect(storageLimits(readOnly)).toEqual({});
  });
});

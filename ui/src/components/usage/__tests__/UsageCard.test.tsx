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

import { ago, GAP, GIB, homeBytes, sessions, targetStats, view } from "./fixtures";

const withQuery = (ui: ReactElement) => <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{ui}</QueryClientProvider>;
afterEach(() => vi.unstubAllGlobals());

const titles = (root: HTMLElement) => Array.from(root.querySelectorAll("svg title")).map((t) => t.textContent ?? "");

describe("What I have used: data", () => {
  it("sums a counter over both windows, one row per series, and shows a gauge's latest with its day", () => {
    render(<UsageCard title={copy.MEMBER_TITLE} metrics={[homeBytes(), sessions()]} limits={{ quota_field: 20 * GIB }} />);
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
    render(<UsageCard title={copy.MEMBER_TITLE} metrics={[homeBytes()]} limits={{ quota_field: 20 * GIB }} />);
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuemax", String(20 * GIB));
    expect(meter).toHaveAttribute("aria-valuenow", String(18.4 * GIB));
    expect(screen.getByText("18.4 GiB of 20.0 GiB")).toBeInTheDocument();
    expect(meter.firstElementChild).toHaveClass("bg-accent");
  });

  it("turns the meter warn at the limit, says no limit for 0, and says it was not read for none", () => {
    const { rerender } = render(<UsageCard title="t" metrics={[homeBytes()]} limits={{ quota_field: 18 * GIB }} />);
    expect(screen.getByRole("meter").firstElementChild).toHaveClass("bg-warn");
    rerender(<UsageCard title="t" metrics={[homeBytes()]} limits={{ quota_field: 0 }} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.getByText(copy.NO_LIMIT)).toBeInTheDocument();
    rerender(<UsageCard title="t" metrics={[homeBytes()]} limits={{}} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.getByText(copy.LIMIT_UNREAD)).toBeInTheDocument();
    // The value is still said, so a missing limit never hides the number.
    expect(screen.getByText(/^18\.4 GiB on/)).toBeInTheDocument();
  });

  it("draws no meter on a totals page, where no limit belongs to anyone", () => {
    render(<UsageCard title="t" metrics={[homeBytes()]} />);
    expect(screen.queryByRole("meter")).toBeNull();
    expect(screen.queryByText(copy.LIMIT_UNREAD)).toBeNull();
  });

  it("draws no meter for a gauge without `against`", () => {
    render(<UsageCard title="t" metrics={[homeBytes({ against: undefined })]} limits={{ quota_field: 20 * GIB }} />);
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("ends on a next step", () => {
    render(<UsageCard title={copy.MEMBER_TITLE} metrics={[sessions()]} />);
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });
});

describe("What I have used: gaps", () => {
  const gapped = [sessions({ gaps: [GAP] })];

  it("footnotes the days and the add-on's reason, quoted, and says a blank is not a zero", () => {
    render(<UsageCard title="t" metrics={gapped} />);
    expect(screen.getByText(copy.GAPS_HEADING)).toBeInTheDocument();
    expect(screen.getByText(/Area: “ring”: “Auditing is off in this area”/)).toBeInTheDocument();
    expect(screen.getByText(copy.GAPS_NOTE)).toBeInTheDocument();
  });

  it("hatches the days and breaks the line: no gap day is a zero, and no gap day has a value", () => {
    const { container } = render(<UsageCard title="t" metrics={gapped} />);
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
    render(<UsageCard title="t" metrics={gapped} />);
    expect(screen.getAllByRole("img")[0]).toHaveAccessibleName(/Days that could not be counted are hatched/);
    expect(screen.getAllByRole("img")[1]).not.toHaveAccessibleName(/hatched/);
  });

  it("a gauge's gap breaks its line too, and a gauge's day with no sample is joined over", () => {
    const gauge = homeBytes({ against: undefined, gaps: [{ start: ago(8), end: ago(5), reason: "The NAS was off" }], series: [{ dimensions: {}, latest: { day: ago(1), value: 5 }, days: [{ day: ago(12), value: 4 }, { day: ago(10), value: 4 }, { day: ago(3), value: 5 }, { day: ago(1), value: 5 }] }] });
    render(<UsageCard title="t" metrics={[gauge]} />);
    const svg = screen.getByRole("img");
    expect(svg.querySelectorAll("[data-line]")).toHaveLength(2);
    expect(svg.querySelectorAll("[data-gap]")).toHaveLength(1);
  });
});

describe("add-on text is quoted, capped and escaped", () => {
  it("caps a long reason and a long label, and never turns markup into elements", () => {
    const long = `<b>x</b>${"y".repeat(400)}`;
    const { container } = render(<UsageCard title="t" metrics={[sessions({ label: long, gaps: [{ ...GAP, reason: long }] })]} />);
    expect(container.querySelector("b")).toBeNull();
    const line = screen.getByText(/Area: “ring”: “<b>x<\/b>y+…”/);
    expect(line.textContent!.length).toBeLessThan(200);
    expect(screen.getByRole("heading").textContent!.length).toBeLessThanOrEqual(60);
  });
});

describe("What I have used: not read yet", () => {
  it("says not counted yet, draws nothing, and its step is what waits", () => {
    const { container } = render(<UsageCard title="t" metrics={[sessions({ watermark: null, series: [] })]} />);
    expect(screen.getByText(copy.NOT_COUNTED)).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeNull();
    expect(screen.getByText(copy.NOT_COUNTED_NEXT)).toBeInTheDocument();
    expect(screen.queryByText(copy.NOTHING_TO_DO)).toBeNull();
  });

  it("says nothing counted for a counted metric with no series: a real zero, not 'not counted'", () => {
    render(<UsageCard title="t" metrics={[sessions({ series: [] })]} />);
    expect(screen.getByText(copy.NOTHING_COUNTED)).toBeInTheDocument();
    expect(screen.queryByText(copy.NOT_COUNTED)).toBeNull();
  });
});

describe("a person's view of a system", () => {
  it("titles the member's card as theirs and the operator's by the person's name", () => {
    const usage = { reported: true as const, metrics: [sessions()] };
    const { rerender } = render(<ViewUsage view={view(usage)} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    rerender(<ViewUsage view={view(usage)} person="Rey Mysterio" />);
    expect(screen.getByRole("group", { name: "What Rey Mysterio has used" })).toBeInTheDocument();
  });

  it("draws nothing where the add-on does not report statistics, or this person has no per-person metric", () => {
    const a = render(<ViewUsage view={view({ reported: false })} />);
    expect(a.container).toBeEmptyDOMElement();
    const b = render(<ViewUsage view={view({ reported: true, metrics: [] })} />);
    expect(b.container).toBeEmptyDOMElement();
    const c = render(<ViewUsage view={view({ reported: true, metrics: [sessions()] }, { kind: undefined })} />);
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
    render(<MemberTargetCard view={view({ reported: true, metrics: [sessions()] })} />);
    expect(screen.getByRole("group", { name: "What I have used" })).toBeInTheDocument();
    expect(screen.getByText("rey_mysterio")).toBeInTheDocument();
    // The card reads what the member's own view carried: it asks for nothing, so it cannot ask for anyone else's.
    expect(fetchStub.calls).toEqual([]);
  });

  it("meters a storage member's gauge over the home limit the view observed, found by the schema's limit field", async () => {
    const { storageLimits } = await import("@/components/kinds/storage/limits");
    const schema = [{ name: "quota_field", role: "limit", unit: "bytes", label: "Limit" }] as never;
    const storage = view({ reported: true, metrics: [homeBytes()] }, { kind: "storage", entitlement_schema: schema });
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
    expect(screen.getByText(/reports more “Sessions” series than Syndra keeps/)).toBeInTheDocument();
    expect(screen.getByText(copy.retention(400, false))).toBeInTheDocument();
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });

  it("says an operator shortened the retention, and counts accounts in the plural", async () => {
    const t = targetStats({ retention: { subject_days: 90, set: true } });
    if (t.usage.reported) t.usage.metrics[0].collected = { at: null, unbound_accounts: 2 };
    stubStats(t);
    render(withQuery(<TargetStats target="gym" />));
    expect(await screen.findByText(copy.retention(90, true))).toBeInTheDocument();
    expect(screen.getByText(/2 accounts Syndra does not manage also used it. They are in the totals./)).toBeInTheDocument();
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

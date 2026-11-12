// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { InsightsUsage } from "@/components/usage/InsightsUsage";
import * as copy from "@/components/usage/copy";
import { ADVANCED_NAV, BASIC_NAV, type NavEntry } from "@/lib/nav";
import type { InsightsStats } from "@/lib/queries/useStats";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { GIB } from "./fixtures";

const stats = (): InsightsStats => ({
  window_days: 30,
  from: "2026-10-14",
  metrics: [
    { target: "gym", system: "Training hall", name: "volume.sessions", instrument: "counter", unit: "count", label: "Sessions", per_subject: true, dimensions: [{ name: "area", label: "Area" }],
      series: [{ dimensions: { area: "ring" }, members: 41, p50: 3, p90: 11.5 }, { dimensions: { area: "mat" }, members: 1, p50: 2, p90: 2 }] },
    { target: "gym", system: "Training hall", name: "volume.used_bytes", instrument: "gauge", unit: "bytes", label: "Home folder used", per_subject: true, against: "quota_field",
      series: [{ dimensions: {}, members: 214, p50: 1.2 * GIB, p90: 9 * GIB }] },
    { target: "lab", system: "Laser lab", name: "laser.minutes", instrument: "counter", unit: "seconds", label: "Machine time", per_subject: true,
      series: [{ dimensions: {}, members: 12, p50: 1800, p90: 7200 }] },
  ],
});

const mount = (body: unknown) => {
  const stub = makeProxyFetch();
  stub.register("GET", "/api/proxy/insights/stats", () => body);
  vi.stubGlobal("fetch", stub.fetchImpl);
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <InsightsUsage />
    </QueryClientProvider>,
  );
  return stub;
};
afterEach(() => vi.unstubAllGlobals());

describe("Insights, Usage", () => {
  it("reads the operators' endpoint and groups by unit, in a fixed order, with members and a spread", async () => {
    const stub = mount(stats());
    expect(await screen.findByText(copy.UNIT_HEADING.count)).toBeInTheDocument();
    expect(stub.calls.map((c) => c.url)).toEqual(["/api/proxy/insights/stats"]);
    const groups = screen.getAllByText(new RegExp(`^(${Object.values(copy.UNIT_HEADING).join("|")})$`)).map((e) => e.textContent);
    expect(groups).toEqual(["Counts", "Sizes", "Durations"]);
    // One heading for a metric, one row per breakdown under it.
    expect(screen.getAllByText("Training hall: Sessions")).toHaveLength(1);
    expect(screen.getByText(/^41 people\./)).toBeInTheDocument();
    expect(screen.getByText(/^1 person\./)).toBeInTheDocument();
    expect(screen.getByText(/Half are at 3 or under; nine in ten are at 11\.5 or under\./)).toBeInTheDocument();
    expect(screen.getByText(/Half are at 1\.2 GiB or under; nine in ten are at 9\.0 GiB or under\./)).toBeInTheDocument();
    expect(screen.getByText(/Half are at 30 minutes or under; nine in ten are at 2 hours or under\./)).toBeInTheDocument();
    expect(screen.getByText(copy.windowFrom("14 Oct"))).toBeInTheDocument();
  });

  it("keeps each breakdown as its own row and never adds them", async () => {
    mount(stats());
    await screen.findByText(/^41 people\./);
    expect(screen.getByText("Area: “ring”")).toBeInTheDocument();
    expect(screen.getByText("Area: “mat”")).toBeInTheDocument();
    expect(screen.queryByText(/^42 people/)).toBeNull();
  });

  it("shows no names, whatever else a row carries, and ranks nothing", async () => {
    const s = stats();
    (s.metrics[0].series[0] as Record<string, unknown>).subject = "Rey Mysterio";
    (s.metrics[0].series[0] as Record<string, unknown>).name = "Randy Orton";
    mount(s);
    await screen.findByText(/^41 people\./);
    expect(document.body.textContent).not.toMatch(/Rey Mysterio|Randy Orton/);
    expect(document.body.textContent).not.toMatch(/\b(top|most|worst|best|leader|rank)/i);
  });

  it("ends on a next step that sends to a person's own page", async () => {
    mount(stats());
    await screen.findByText(/^41 people\./);
    expect(screen.getByText(copy.INSIGHTS_NEXT)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: new RegExp(copy.INSIGHTS_NEXT_LINK) })).toHaveAttribute("href", "/users");
  });

  it("says nothing reports statistics yet, with its step, when no system does", async () => {
    mount({ window_days: 30, from: "2026-10-14", metrics: [] });
    expect(await screen.findByText(copy.INSIGHTS_EMPTY)).toBeInTheDocument();
  });

  it("caps the add-on's text: a long system name and label are cut", async () => {
    const s = stats();
    s.metrics[0].system = "S".repeat(200);
    s.metrics[0].label = "L".repeat(200);
    mount(s);
    const row = await screen.findByText(/^S+…: L+…$/);
    expect(row.textContent!.length).toBeLessThan(130);
  });
});

describe("Insights is in the Advanced view only", () => {
  const hrefs = (nav: NavEntry[]): string[] => nav.flatMap((e) => (e.kind === "leaf" ? [e.href] : e.children.map((c) => c.href)));
  it("is in Advanced's navigation and not in Basic's", () => {
    expect(hrefs(ADVANCED_NAV)).toContain("/insights");
    expect(hrefs(BASIC_NAV)).not.toContain("/insights");
  });
});


// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { InsightsUsage } from "@/components/usage/InsightsUsage";
import * as copy from "@/components/usage/copy";
import { UNKNOWN_ACCOUNT } from "@/components/names/Unresolved";
import { ADVANCED_NAV, BASIC_NAV, type NavEntry } from "@/lib/nav";
import type { InsightsStats } from "@/lib/queries/useStats";
import { dayWords } from "@/lib/usage";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { ago, GIB } from "./fixtures";

const person = (user_id: string, name: string, last_30d: number | null, last_12m: number) => ({ user_id, name, last_30d, last_12m });

const stats = (): InsightsStats => ({
  window_days: 30,
  from: ago(29),
  from_12m: ago(300),
  metrics: [
    { target: "gym", system: "Training hall", name: "volume.sessions", instrument: "counter", unit: "count", label: "Sessions", per_subject: true, dimensions: [{ name: "area", label: "Area" }], window_30d_from: ago(29), window_12m_from: ago(300),
      series: [
        { dimensions: { area: "ring" }, members: 41, p50: 3, p90: 11.5,
          people: [person("sub-lita", "Lita", 3, 30), person("sub-edge", "Edge", 12, 14), person("sub-trish", "Trish Stratus", 0, 50)] },
        { dimensions: { area: "mat" }, members: 1, p50: 2, p90: 2, people: [person("sub-kane", "Kane", 2, 2)] },
      ] },
    { target: "gym", system: "Training hall", name: "volume.used_bytes", instrument: "gauge", unit: "bytes", label: "Home folder used", per_subject: true, against: "quota_field", window_30d_from: ago(29), window_12m_from: ago(300),
      series: [{ dimensions: {}, members: 214, p50: 1.2 * GIB, p90: 9 * GIB, people: [person("sub-batista", "Batista", null, 2 * GIB), person("sub-orton", "", 4 * GIB, 4 * GIB)] }] },
    { target: "lab", system: "Laser lab", name: "laser.minutes", instrument: "counter", unit: "seconds", label: "Machine time", per_subject: true, window_30d_from: ago(29), window_12m_from: ago(300),
      series: [{ dimensions: {}, members: 12, p50: 1800, p90: 7200, people: [person("sub-rey", "Rey Mysterio", 1800, 3600)] }] },
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
    expect(screen.getByText(copy.windowFrom(dayWords(ago(29))))).toBeInTheDocument();
  });

  it("shows the spread and its bar for any number of people, one included: an operator's read has no floor", async () => {
    mount(stats());
    const one = (await screen.findByText(/^1 person\./)).closest(".row-divider") as HTMLElement;
    expect(one.textContent).toContain("Half are at 2 or under; nine in ten are at 2 or under.");
    expect(one.querySelector("[aria-hidden]")).not.toBeNull();
  });

  it("says nobody used it in the 30 days, and draws no bar, when its people are from the 12 months only", async () => {
    const s = stats();
    s.metrics[2].series[0] = { ...s.metrics[2].series[0], members: 0, p50: 0, p90: 0, people: [person("sub-rey", "Rey Mysterio", 0, 3600)] };
    mount(s);
    const row = (await screen.findByText(copy.NOBODY_RECENTLY)).closest(".row-divider") as HTMLElement;
    expect(row.textContent).not.toMatch(/Half are at/);
    expect(row.querySelector("[aria-hidden]")).toBeNull();
  });

  it("says a spread is over fewer days when the backend started it later than the window, and says nothing otherwise", async () => {
    const s = stats();
    s.metrics[0].window_30d_from = ago(9); // a system keeping 10 days: the backend's own clamp
    s.metrics[0].window_12m_from = ago(9);
    mount(s);
    await screen.findByText(/^41 people\./);
    expect(screen.getAllByText(copy.spreadDays(10))).toHaveLength(1);
    expect(screen.queryByText(/^Over the last/, { selector: "p" })?.textContent).toBe(copy.spreadDays(10));
    expect(screen.queryByText(copy.spreadDays(30))).toBeNull();
    expect(screen.getAllByText(copy.yearDays(10))).toHaveLength(1);
  });

  it("says the 12-month numbers are over fewer days when only that window was cut short", async () => {
    const s = stats();
    s.metrics[0].window_12m_from = ago(89); // a system keeping 90 days
    mount(s);
    await screen.findByText(/^41 people\./);
    expect(screen.getAllByText(copy.yearDays(90))).toHaveLength(1);
    expect(screen.queryByText(/^Over the last/, { selector: "p" })).toBeNull();
  });

  it("keeps each breakdown as its own row and never adds them", async () => {
    mount(stats());
    await screen.findByText(/^41 people\./);
    expect(screen.getByText("Area: “ring”")).toBeInTheDocument();
    expect(screen.getByText("Area: “mat”")).toBeInTheDocument();
    expect(screen.queryByText(/^42 people/)).toBeNull();
  });

  it("names each breakdown's people, largest in the last 30 days first, each linking to their page", async () => {
    mount(stats());
    const table = await screen.findByRole("table", { name: "Training hall: Sessions, Area: “ring”: each person's numbers" });
    const names = within(table).getAllByRole("link").map((a) => [a.textContent, a.getAttribute("href")]);
    expect(names).toEqual([["Edge", "/users/sub-edge"], ["Lita", "/users/sub-lita"], ["Trish Stratus", "/users/sub-trish"]]);
    const edge = within(table).getAllByRole("row")[1];
    expect(within(edge).getAllByRole("cell").map((c) => c.textContent)).toEqual(["Edge", `${copy.LAST_30_COLUMN}12`, `${copy.LAST_12_COLUMN}14`]);
  });

  it("re-sorts by each heading, and a second press turns the order", async () => {
    mount(stats());
    const table = await screen.findByRole("table", { name: /Area: “ring”/ });
    const order = () => within(table).getAllByRole("link").map((a) => a.textContent);
    fireEvent.click(within(table).getByRole("button", { name: "Sort by last 12 months" }));
    await waitFor(() => expect(order()).toEqual(["Trish Stratus", "Lita", "Edge"]));
    fireEvent.click(within(table).getByRole("button", { name: "Sort by person" }));
    await waitFor(() => expect(order()).toEqual(["Edge", "Lita", "Trish Stratus"]));
    fireEvent.click(within(table).getByRole("button", { name: "Sort by person" }));
    await waitFor(() => expect(order()).toEqual(["Trish Stratus", "Lita", "Edge"]));
  });

  it("puts a gauge with no value in the 30 days after every value, either way, and says so in its cell", async () => {
    mount(stats());
    const table = await screen.findByRole("table", { name: /Home folder used: each/ });
    const first = () => within(table).getAllByRole("row")[1].textContent;
    expect(first()).toContain("4.0 GiB");
    expect(within(table).getAllByRole("row")[2].textContent).toContain(copy.NO_VALUE);
    fireEvent.click(within(table).getByRole("button", { name: "Sort by last 30 days" }));
    await waitFor(() => expect(within(table).getAllByRole("row")[2].textContent).toContain(copy.NO_VALUE));
  });

  it("never labels a person with their id: a name the directory could not give is said as such, the id only beside it", async () => {
    mount(stats());
    const table = await screen.findByRole("table", { name: /Home folder used: each/ });
    const orton = within(table).getAllByRole("row")[1];
    expect(within(orton).getAllByRole("cell")[0].textContent).toMatch(new RegExp(`^${UNKNOWN_ACCOUNT}`));
  });

  it("ranks nothing in words", async () => {
    mount(stats());
    await screen.findByText(/^41 people\./);
    expect(document.body.textContent).not.toMatch(/\b(top|most|worst|best|leader|rank)/i);
  });

  it("ends on a next step that sends to a person's own page", async () => {
    mount(stats());
    await screen.findByText(/^41 people\./);
    expect(screen.getByText(copy.INSIGHTS_NEXT)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: new RegExp(copy.INSIGHTS_NEXT_LINK) })).toHaveAttribute("href", "/users");
  });

  it("says nothing reports statistics yet, with its step, when no system does", async () => {
    mount({ window_days: 30, from: "2026-10-14", from_12m: "2025-12-01", metrics: [] });
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


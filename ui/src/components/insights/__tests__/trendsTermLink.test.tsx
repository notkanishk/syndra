// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const respond = vi.fn();
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: (path: string) => respond(path),
}));
vi.mock("@/lib/queries/useTopology", () => ({ useTopology: () => ({ data: undefined }) }));

let query = "";
vi.mock("next/navigation", () => ({
  usePathname: () => "/insights",
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(query),
}));

import { TrendsTab } from "@/components/insights/TrendsTab";
import { useInsightsUrl } from "@/components/insights/useInsightsUrl";

// One write place per setting: where Trends shows a term it says where the
// dates are set, and only where it is showing a term.

function Mount() {
  return <TrendsTab url={useInsightsUrl()} />;
}

const trends = (label: string) => ({
  window: { from: "2026-08-01", to: "2026-12-31", label },
  weekly: [],
  totals: { given: 0, taken_away: 0 },
  requests: { came_in: 0, approved: 0, declined: 0, withdrawn: 0, declined_reasons: [], median_hours_to_decide: null, outside_bundles: [] },
  sending: { median_minutes_approval_to_sent: null, sample: 0 },
  drift: { found: 0, cleared: 0, median_hours_to_clear: null },
  ending_soon: [],
  upkeep: { people_who_left_holding: 0, drift_open: 0, roles_nobody_has: 0, bundles_nothing_new: 0, overlaps: 0 },
});

function mount(q: string, label: string) {
  query = q;
  respond.mockResolvedValue(trends(label));
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <Mount />
    </QueryClientProvider>,
  );
}

afterEach(cleanup);

// A-07 §17: Ending soon is the product's one window, two columns, with the arrow on the
// column its rows are in order by.
describe("Ending soon", () => {
  it("has two columns, 14 days and 30 days, and never 60 or 90", async () => {
    query = "tab=trends";
    respond.mockResolvedValue({
      ...trends("Monsoon 2026"),
      ending_soon: [
        { project_id: "p1", project_name: "Laser room", d14: 1, d30: 2 },
        { project_id: "p2", project_name: "Wood shop", d14: 2, d30: 5 },
      ],
    });
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <Mount />
      </QueryClientProvider>,
    );
    await screen.findByText("Laser room");
    expect(screen.getAllByRole("columnheader").map((h) => h.textContent)).toEqual(["Project", "14 days", "30 days ↑"]);
    expect(screen.getByRole("table", { name: "Ending soon, by project" }).textContent).not.toMatch(/60|90/);
    // In order by the 30-day count, most first (the first row is the headings).
    const rows = screen.getAllByRole("row").slice(1).map((r) => r.textContent);
    expect(rows).toEqual(["Wood shop25", "Laser room12"]);
  });

  it("opens Expiring access in the same window from each number", async () => {
    query = "tab=trends";
    respond.mockResolvedValue({
      ...trends("Monsoon 2026"),
      ending_soon: [{ project_id: "p1", project_name: "Laser room", d14: 1, d30: 2 }],
    });
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <Mount />
      </QueryClientProvider>,
    );
    await screen.findByText("Laser room");
    expect(screen.getByRole("link", { name: "1 in 14 days: open Expiring access" })).toHaveAttribute("href", "/review/expiring-access?within=14");
    expect(screen.getByRole("link", { name: "2 in 30 days: open Expiring access" })).toHaveAttribute("href", "/review/expiring-access?within=30");
  });

});

describe("Trends and the term dates", () => {
  it("links to Makerspace settings beside the term it shows", async () => {
    mount("tab=trends", "Monsoon 2026");
    await screen.findByText(/This term is Monsoon 2026/);
    const link = screen.getByRole("link", { name: /Set in Makerspace settings/ });
    expect(link.getAttribute("href")).toBe("/settings");
  });

  it("says which term it is reading, with its dates, ahead of the link", async () => {
    mount("tab=trends", "Monsoon 2026");
    expect(await screen.findByText(/This term is Monsoon 2026, 1 Aug to\s+31 Dec/)).toBeInTheDocument();
  });

  it("says nothing about term dates when the period is not a term", async () => {
    mount("tab=trends&period=30d", "Last 30 days");
    await screen.findByText("Last 30 days");
    expect(screen.queryByRole("link", { name: /Makerspace settings/ })).toBeNull();
  });
});

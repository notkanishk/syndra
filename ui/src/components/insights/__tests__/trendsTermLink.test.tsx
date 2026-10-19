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

describe("Trends and the term dates", () => {
  it("links to Makerspace settings beside the term it shows", async () => {
    mount("tab=trends", "Monsoon 2026");
    await screen.findByText("Monsoon 2026");
    const link = screen.getByRole("link", { name: /Set in Makerspace settings/ });
    expect(link.getAttribute("href")).toBe("/settings");
  });

  it("says nothing about term dates when the period is not a term", async () => {
    mount("tab=trends&period=30d", "Last 30 days");
    await screen.findByText("Last 30 days");
    expect(screen.queryByRole("link", { name: /Makerspace settings/ })).toBeNull();
  });
});

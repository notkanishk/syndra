// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const respond = vi.fn();
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: (path: string) => respond(path),
}));
vi.mock("@/lib/queries/useTopology", () => ({ useTopology: () => ({ data: undefined }) }));

let query = "";
const replace = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => "/insights",
  useRouter: () => ({ replace }),
  useSearchParams: () => new URLSearchParams(query),
}));

import { asOfPanel } from "@/components/insights/asof";
import { TrendsTab } from "@/components/insights/TrendsTab";
import { useInsightsUrl } from "@/components/insights/useInsightsUrl";
import { ApiError } from "@/lib/api-client";
import { useInsightsAsOf, type InsightsTrends } from "@/lib/queries/useInsights";

const wrap = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider>
);

const trends = (over: Partial<InsightsTrends> = {}): InsightsTrends => ({
  window: { from: "2026-08-01", to: "2026-12-31", label: "Monsoon 2026" },
  weekly: [{ week_start: "2026-10-12", through_bundle: 2, by_request: 1, by_hand: 0, taken_away: 1 }],
  totals: { given: 3, taken_away: 1 },
  requests: { came_in: 4, approved: 2, declined: 1, withdrawn: 1, declined_reasons: [], median_hours_to_decide: 5, outside_bundles: [] },
  sending: { median_minutes_approval_to_sent: null, sample: 0 },
  drift: { found: 2, cleared: 1, median_hours_to_clear: null },
  ending_soon: [],
  upkeep: { people_who_left_holding: 0, drift_open: 1, roles_nobody_has: 2, bundles_nothing_new: 0, overlaps: 1 },
  ...over,
});

function Mount() {
  return <TrendsTab url={useInsightsUrl()} />;
}

beforeEach(() => {
  respond.mockReset();
  replace.mockReset();
  query = "tab=trends";
});

describe("Trends tab", () => {
  it("shows every card with the period control in place, and says delivery history is only just starting", async () => {
    respond.mockResolvedValue(trends());
    render(<Mount />, { wrapper: wrap });

    expect(screen.getByRole("radio", { name: "This term" })).toBeChecked();
    await screen.findByText("Delivery history starts being kept from now.");
    for (const title of ["Access given and revoked, by week", "Requests", "Delivery", "Drift", "Ending soon", "Upkeep"]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
    expect(screen.getByText("3 given, 1 revoked")).toBeInTheDocument();
    expect(document.body.textContent?.toLowerCase()).not.toContain("taken away");
    expect(respond).toHaveBeenCalledWith("/insights/trends?period=term");
  });

  it("states the delivery time once there is a history", async () => {
    respond.mockResolvedValue(trends({ sending: { median_minutes_approval_to_sent: 12, sample: 3 } }));
    render(<Mount />, { wrapper: wrap });
    await screen.findByText("12 minutes");
    expect(screen.queryByText("Delivery history starts being kept from now.")).not.toBeInTheDocument();
  });

  it("puts the chosen period in the link, and leaves the default out of it", async () => {
    respond.mockResolvedValue(trends());
    render(<Mount />, { wrapper: wrap });
    await screen.findByText("Delivery");
    fireEvent.click(screen.getByRole("radio", { name: "30 days" }));
    expect(replace).toHaveBeenCalledWith("/insights?tab=trends&period=30d", { scroll: false });
  });
});

describe("as of a date in the link", () => {
  it("reads a real past date and nothing else", () => {
    for (const [q, want] of [
      ["asof=2020-02-29", "2020-02-29"],
      ["asof=2020-02-30", null],
      ["asof=yesterday", null],
      ["asof=2999-01-01", null],
      ["", null],
    ] as const) {
      query = q;
      expect(renderHook(() => useInsightsUrl()).result.current.asof, q).toBe(want);
    }
  });

  it("writes the date, and clears it to come back to today", () => {
    query = "tab=trends&asof=2020-02-29";
    const { result } = renderHook(() => useInsightsUrl());
    result.current.set({ asof: null });
    expect(replace).toHaveBeenCalledWith("/insights?tab=trends", { scroll: false });
    result.current.set({ asof: "2020-03-01" });
    expect(replace).toHaveBeenLastCalledWith("/insights?tab=trends&asof=2020-03-01", { scroll: false });
  });

  it("reads the period, defaulting to the term for anything it does not know", () => {
    query = "period=year";
    expect(renderHook(() => useInsightsUrl()).result.current.period).toBe("year");
    query = "period=week";
    expect(renderHook(() => useInsightsUrl()).result.current.period).toBe("term");
  });
});

describe("a date with no picture", () => {
  it("is data, not a failure, and carries the first day", async () => {
    respond.mockRejectedValue(new ApiError(404, { error: "NO_SNAPSHOT", message: "none", details: { first_day: "2026-10-16" } }));
    const { result } = renderHook(() => useInsightsAsOf("2026-10-01"), { wrapper: wrap });
    await waitFor(() => expect(result.current.data).toEqual({ kind: "none", first_day: "2026-10-16" }));
  });

  it("says when the pictures start", () => {
    const panel = asOfPanel("2026-10-01", { kind: "none", first_day: "2026-10-16" }, { model: null, held: new Map() }, new Map());
    expect(panel.lines[0].text).toBe("Syndra keeps one picture a day, starting 16 Oct 2026. There's none for that date.");
  });

  it("says to come back tomorrow when there is no picture at all", () => {
    const panel = asOfPanel("2026-10-01", { kind: "none", first_day: null }, { model: null, held: new Map() }, new Map());
    expect(panel.lines[0].text).toBe("Syndra keeps one picture a day, starting today. Come back tomorrow for the first earlier date.");
  });
});

// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TouchNav } from "@/components/shell/TouchNav";
import type { Indicators } from "@/lib/queries/useIndicators";

const pathname = vi.hoisted(() => ({ value: "/" }));
const view = vi.hoisted(() => ({ audience: "basic" as string, isOperator: true }));
const indicators = vi.hoisted(() => ({
  data: undefined as Indicators | undefined,
  isPlaceholderData: false,
}));
const targets = vi.hoisted(() => ({ data: [] as Array<{ target: string }> }));

vi.mock("next/navigation", () => ({ usePathname: () => pathname.value }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ ...view, view: "basic", setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => targets }));
vi.mock("@/lib/queries/useIndicators", () => ({ useIndicators: () => indicators }));

function renderNav() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TouchNav />
    </QueryClientProvider>,
  );
}

function tabLabels(): string[] {
  return within(screen.getByRole("navigation", { name: "Primary" }))
    .getAllByRole("link")
    .map((link) => link.textContent?.replace(/\d+$/, "").replace(/Needs attention/, "").trim() ?? "");
}

beforeEach(() => {
  pathname.value = "/";
  view.audience = "basic";
  view.isOperator = true;
  indicators.data = undefined;
  indicators.isPlaceholderData = false;
  targets.data = [];
});

describe("the tab bar carries the rail's own tree", () => {
  it("gives a member three tabs and no view switch", () => {
    view.audience = "member";
    view.isOperator = false;
    renderNav();

    expect(tabLabels()).toEqual(["My access", "Requests", "Network storage"]);
    expect(screen.queryByRole("button", { name: /go to|advanced/i })).toBeNull();
  });

  // Basic has four entries and six leaves. Six tabs would be the rail again.
  it("gives Basic four tabs, the third keeping the group's own word", () => {
    renderNav();
    expect(tabLabels()).toEqual(["Home", "People", "Access", "Requests"]);
  });

  it("marks the group current when any of its children is the page", () => {
    pathname.value = "/roles";
    renderNav();
    const access = screen.getByRole("link", { name: /Access/ });
    expect(access).toHaveAttribute("aria-current", "page");
  });

  it("lands the group's tab on its first child rather than nowhere", () => {
    renderNav();
    expect(screen.getByRole("link", { name: /Access/ })).toHaveAttribute("href", "/projects");
  });

  it("shows a count on a destination that has one", () => {
    indicators.data = { pending_requests: 4 } as Indicators;
    renderNav();
    expect(within(screen.getByRole("link", { name: /Requests/ })).getByText("4")).toBeTruthy();
  });

  // Until the first payload lands the counts are the query's placeholder
  // zeros, and a badge drawn from them is a claim nobody made.
  it("shows no count while the read is still the placeholder", () => {
    indicators.data = { pending_requests: 4 } as Indicators;
    indicators.isPlaceholderData = true;
    renderNav();
    expect(within(screen.getByRole("link", { name: /Requests/ })).queryByText("4")).toBeNull();
  });
});

describe("Advanced does not get a tab bar", () => {
  beforeEach(() => {
    view.audience = "advanced";
  });

  it("offers one bar naming where you are", () => {
    pathname.value = "/bundles";
    renderNav();
    expect(screen.getByRole("button", { name: /Bundles/ })).toBeTruthy();
  });

  // Three findings plus eleven expiries plus three holds is seventeen of
  // nothing — three kinds of work, no action that reduces the number.
  it("counts places needing attention, not items outstanding", () => {
    indicators.data = { drift: 3, expiring_grants: 11, holds_due: 3 } as Indicators;
    renderNav();
    expect(screen.getByText("1 place needs attention")).toBeTruthy();
    expect(screen.queryByText(/17/)).toBeNull();
  });

  it("opens the rail as a sheet, and closes it when a destination is picked", () => {
    renderNav();
    fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));

    const sheet = screen.getByRole("dialog", { name: "Go to" });
    expect(within(sheet).getByRole("link", { name: /Bundles/ })).toBeTruthy();

    fireEvent.click(within(sheet).getByRole("link", { name: /Bundles/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("expands only the section you are in", () => {
    pathname.value = "/governance/drift";
    renderNav();
    fireEvent.click(screen.getByRole("button", { name: /Review/ }));
    const sheet = screen.getByRole("dialog", { name: "Go to" });

    // Review is where we are, so its children are listed.
    expect(within(sheet).getByRole("link", { name: /Withdrawn access/ })).toBeTruthy();
    // Automation is not, so it is one row rather than five.
    expect(within(sheet).queryByRole("link", { name: /Change history/ })).toBeNull();
  });

  it("keeps a seat for a destination with nothing outstanding", () => {
    pathname.value = "/governance/drift";
    indicators.data = { drift: 0 } as Indicators;
    renderNav();
    fireEvent.click(screen.getByRole("button", { name: /Review/ }));
    const sheet = screen.getByRole("dialog", { name: "Go to" });
    const row = within(sheet).getByRole("link", { name: /Unexplained access/ });
    expect(within(row).getByText("0"), "a hollow zero, not a vanished row").toBeTruthy();
  });
});

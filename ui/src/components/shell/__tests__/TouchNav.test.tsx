// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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
  return within(screen.getByRole("navigation", { name: "Main navigation" }))
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

    const sheet = screen.getByRole("dialog", { name: "Go to a page" });
    expect(within(sheet).getByRole("link", { name: /Bundles/ })).toBeTruthy();

    fireEvent.click(within(sheet).getByRole("link", { name: /Bundles/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("expands only the section you are in", () => {
    pathname.value = "/governance/drift";
    renderNav();
    fireEvent.click(screen.getByRole("button", { name: /Review/ }));
    const sheet = screen.getByRole("dialog", { name: "Go to a page" });

    // Review is where we are, so its children are listed.
    expect(within(sheet).getByRole("link", { name: /Unfinished revocations/ })).toBeTruthy();
    // Automation is not, so it is one row rather than five.
    expect(within(sheet).queryByRole("link", { name: /Change history/ })).toBeNull();
  });

  it("keeps a seat for a destination with nothing outstanding", () => {
    pathname.value = "/governance/drift";
    indicators.data = { drift: 0 } as Indicators;
    renderNav();
    fireEvent.click(screen.getByRole("button", { name: /Review/ }));
    const sheet = screen.getByRole("dialog", { name: "Go to a page" });
    const row = within(sheet).getByRole("link", { name: /Unexplained access/ });
    expect(within(row).getByText("0"), "a hollow zero, not a vanished row").toBeTruthy();
  });
});

/**
 * The sheet pushes one history entry so the system back gesture closes it
 * before it leaves the screen. Both halves of that bargain are tested here:
 * exactly one entry however long the sheet stays open, and the entry spent
 * rather than abandoned when the sheet is dismissed by hand.
 */
describe("the sheet is one level of history", () => {
  // The sheet is Advanced's entry to the rail; Basic has no trigger for it.
  beforeEach(() => {
    view.audience = "advanced";
  });

  // These spy on `window.history`, which outlives the test. Without this the
  // second spy in the file finds the first one still installed and counts its
  // calls too.
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function renderPolling() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    // A fresh element each time: React bails out of a re-render given the same
    // element reference, and this helper exists to force the re-render.
    const tree = () => (
      <QueryClientProvider client={client}>
        <TouchNav />
      </QueryClientProvider>
    );
    const utils = render(tree());
    return { ...utils, poll: () => utils.rerender(tree()) };
  }

  function openSheet() {
    fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
    return screen.getByRole("dialog", { name: "Go to a page" });
  }

  it("pushes one entry however often the indicator poll re-renders", () => {
    const push = vi.spyOn(window.history, "pushState");
    const { poll } = renderPolling();
    openSheet();
    expect(push).toHaveBeenCalledTimes(1);

    // Every 30 seconds `useIndicators` lands and this whole tree re-renders,
    // handing the sheet a fresh `onClose`. That used to re-run the push.
    indicators.data = { drift: 2 } as Indicators;
    poll();
    poll();

    expect(push).toHaveBeenCalledTimes(1);
  });

  it("spends the entry when the grabber dismisses the sheet", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    renderPolling();
    const sheet = openSheet();

    fireEvent.click(within(sheet).getByRole("button", { name: "Close this sheet" }));

    expect(back, "closing directly would leave a dead entry behind").toHaveBeenCalledTimes(1);
  });

  it("spends the entry when the scrim dismisses the sheet", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    renderPolling();
    const sheet = openSheet();

    fireEvent.click(sheet);

    expect(back).toHaveBeenCalledTimes(1);
  });

  it("closes on the back gesture itself", () => {
    renderPolling();
    openSheet();

    fireEvent(window, new PopStateEvent("popstate"));

    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("leaves history alone when a destination is picked, so the tap navigates", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    renderPolling();
    const sheet = openSheet();

    fireEvent.click(within(sheet).getByRole("link", { name: /Bundles/ }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(back).not.toHaveBeenCalled();
  });
});

/**
 * The grabber is a redundant affordance — Esc, the scrim and the system back
 * gesture all dismiss — and it is the panel's first child. Both facts about it
 * are asserted here because both were wrong: it took the focus the trap gives
 * on open, so the sheet opened with the cursor on "close it", and its target
 * was 22px in a product whose floor is 44.
 */
describe("the sheet's grabber", () => {
  beforeEach(() => {
    view.audience = "advanced";
  });

  function openSheet() {
    fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
    return screen.getByRole("dialog", { name: "Go to a page" });
  }

  it("does not take the focus the sheet gives on open", () => {
    renderNav();
    const sheet = openSheet();
    const grabber = within(sheet).getByRole("button", { name: "Close this sheet" });

    expect(grabber.tabIndex).toBe(-1);
    expect(document.activeElement).not.toBe(grabber);
  });

  it("clears the touch floor", () => {
    renderNav();
    const sheet = openSheet();

    expect(within(sheet).getByRole("button", { name: "Close this sheet" }).className).toContain("h-11");
  });

  // Modal's grabber answers to the same word. Two sheets whose handles have
  // different names are two sheets to anything querying by accessible name.
  it("answers to the same word as every other sheet's handle", () => {
    renderNav();
    const sheet = openSheet();

    expect(within(sheet).queryByRole("button", { name: "Close" })).toBeNull();
    expect(within(sheet).getByRole("button", { name: "Close this sheet" })).toBeTruthy();
  });
});

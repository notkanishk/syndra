// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TouchNav } from "@/components/shell/TouchNav";

/**
 * A-08 §3 on the phone: the sheet draws Advanced's whole tree and folds what
 * Basic lacks, so switching inside it moves only the parts that come and go.
 * The bar's shape still follows the view it is in (tabs in Basic, a sheet in
 * Advanced); only an open sheet is held, so it can fold its rows away.
 */

const view = vi.hoisted(() => ({ audience: "advanced" as string }));

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({
    audience: view.audience,
    isOperator: true,
    view: view.audience,
    setView: () => {},
    revealInAdvanced: () => {},
  }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicatorsRead: () => ({ data: undefined, isPlaceholderData: false }),
}));

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const nav = () => (
  <QueryClientProvider client={client}>
    <TouchNav />
  </QueryClientProvider>
);

const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));

beforeEach(() => {
  view.audience = "advanced";
  Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
});
afterEach(() => {
  delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
});

describe("the phone navigation sheet switching view", () => {
  it("keeps the sheet open and folds Advanced's rows away when the view goes to Basic", () => {
    const { rerender } = render(nav());
    fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
    const sheet = screen.getByRole("dialog", { name: "Go to a page" });
    const home = screen.getByRole("link", { name: "Home" });
    const bundles = screen.getByRole("link", { name: "Bundles" });
    // Open at first paint of the sheet: simply there.
    expect(bundles.closest(".unfold")).toHaveAttribute("data-phase", "open");

    view.audience = "basic";
    rerender(nav());

    expect(screen.getByRole("dialog", { name: "Go to a page" })).toBe(sheet);
    expect(screen.getByRole("link", { name: "Home" })).toBe(home);
    expect(home.closest(".unfold")).toBeNull();
    const folding = bundles.closest(".unfold")!;
    expect(folding).toHaveAttribute("data-phase", "closing");
    animationEnd(folding);
    expect(screen.queryByRole("link", { name: "Bundles" })).toBeNull();
  });

  it("opens them again by Unfold when the view returns to Advanced", () => {
    const { rerender } = render(nav());
    fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
    view.audience = "basic";
    rerender(nav());
    animationEnd(screen.getByRole("link", { name: "Bundles" }).closest(".unfold")!);

    view.audience = "advanced";
    rerender(nav());
    expect(screen.getByRole("link", { name: "Bundles" }).closest(".unfold")).toHaveAttribute(
      "data-phase",
      "opening",
    );
  });

  it("is still tabs in Basic when no sheet is open", () => {
    view.audience = "basic";
    render(nav());
    expect(screen.queryByRole("button", { name: /Go to|Home/ })).toBeNull();
    expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();
  });
});

// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import Sidebar from "@/components/shell/Sidebar";

/**
 * A-08 §3: the rail draws Advanced's whole tree for an operator and folds what
 * Basic lacks, so switching moves only the parts that come and go.
 */

const view = vi.hoisted(() => ({ audience: "basic" as string, isOperator: true }));

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ ...view, view: "basic", setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicators: () => ({ data: undefined, isPlaceholderData: false }),
}));

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const rail = () => (
  <QueryClientProvider client={client}>
    <Sidebar />
  </QueryClientProvider>
);

const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));
const region = (label: string) => screen.getByText(label).closest(".unfold") as HTMLElement | null;

beforeEach(() => {
  view.audience = "basic";
  view.isOperator = true;
  Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
});
afterEach(() => {
  delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
});

describe("the rail switching view", () => {
  it("draws only Basic's rows in Basic, with Advanced's folded shut", () => {
    render(rail());
    expect(screen.getByRole("link", { name: "People" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Bundles" })).toBeNull();
    expect(screen.queryByText("Review")).toBeNull();
  });

  it("opens each Advanced part by Unfold and keeps the rows both views share, never remounted", () => {
    const { rerender } = render(rail());
    const home = screen.getByRole("link", { name: "Home" });
    const people = screen.getByRole("link", { name: "People" });
    const projects = screen.getByRole("link", { name: "Projects" });

    view.audience = "advanced";
    rerender(rail());

    expect(region("Bundles")).toHaveAttribute("data-phase", "opening");
    // A whole group opens as one part, its label and rows together.
    expect(region("Review")).toHaveAttribute("data-phase", "opening");
    expect(region("Review")).toContainElement(screen.getByRole("link", { name: /^Drift/ }));
    // What was on screen holds still, and is not inside an Unfold.
    expect(screen.getByRole("link", { name: "Home" })).toBe(home);
    expect(screen.getByRole("link", { name: "People" })).toBe(people);
    expect(screen.getByRole("link", { name: "Projects" })).toBe(projects);
    expect(home.closest(".unfold")).toBeNull();
    expect(projects.closest(".unfold")).toBeNull();
  });

  it("fades and folds them again on the way back to Basic, then they are gone", () => {
    view.audience = "advanced";
    const { rerender } = render(rail());
    // Present at first paint, so simply there: nothing opens.
    expect(region("Bundles")).toHaveAttribute("data-phase", "open");
    const home = screen.getByRole("link", { name: "Home" });

    view.audience = "basic";
    rerender(rail());
    const bundles = region("Bundles")!;
    expect(bundles).toHaveAttribute("data-phase", "closing");
    animationEnd(bundles);
    expect(screen.queryByRole("link", { name: "Bundles" })).toBeNull();
    expect(screen.getByRole("link", { name: "Home" })).toBe(home);
  });

  it("appears and goes at once for a reader who asked for less motion", async () => {
    const { setMediaQuery } = await import("@/test-utils/media");
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(rail());

    view.audience = "advanced";
    rerender(rail());
    expect(region("Bundles")).toHaveAttribute("data-phase", "open");

    view.audience = "basic";
    rerender(rail());
    expect(screen.queryByRole("link", { name: "Bundles" })).toBeNull();
  });

  it("gives a member the rail it had, with nothing folding", () => {
    view.audience = "member";
    view.isOperator = false;
    render(rail());
    expect(screen.getByRole("link", { name: "My access" })).toBeInTheDocument();
    expect(document.querySelector(".unfold")).toBeNull();
  });
});

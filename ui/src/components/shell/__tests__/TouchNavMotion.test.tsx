// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TouchNav } from "@/components/shell/TouchNav";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/**
 * Motion v2 S1 and S3 on the phone navigation sheet: it leaves the way a dialog
 * does, and its grabber drags it the way a dialog's does. The pieces are Modal's
 * own; these hold the nav sheet to them.
 */

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({
    audience: "advanced",
    isOperator: true,
    view: "advanced",
    setView: () => {},
    revealInAdvanced: () => {},
  }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicators: () => ({ data: undefined, isPlaceholderData: false }),
}));

const ghost = () => document.body.querySelector<HTMLElement>(".leave-scrim");

function open() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(
    <QueryClientProvider client={client}>
      <TouchNav />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
  const scrim = screen.getByRole("dialog", { name: "Go to a page" });
  const panel = scrim.firstElementChild as HTMLElement;
  vi.spyOn(panel, "getBoundingClientRect").mockReturnValue({ height: 600 } as DOMRect);
  const grabber = within(scrim).getByRole("button", { name: "Close this sheet" });
  return { ...view, scrim, panel, grabber };
}

function drag(grabber: HTMLElement, to: number, over = 300) {
  fireEvent.pointerDown(grabber, { clientY: 100, pointerId: 1 });
  act(() => void vi.advanceTimersByTime(over));
  fireEvent.pointerMove(grabber, { clientY: 100 + to, pointerId: 1 });
  return () => fireEvent.pointerUp(grabber, { clientY: 100 + to, pointerId: 1 });
}

beforeEach(() => {
  document.querySelectorAll(".leave-scrim").forEach((copy) => copy.remove());
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("the navigation sheet closing (S1)", () => {
  it("leaves a copy that fades and sinks, then the scrim 40ms behind, then nothing", () => {
    open();
    expect(ghost()).toBeNull();

    // The system back gesture is how a sheet is closed once its entry is spent.
    fireEvent(window, new PopStateEvent("popstate"));

    expect(screen.queryByRole("dialog")).toBeNull();
    const copy = ghost()!;
    expect(copy).not.toBeNull();
    expect(copy).toHaveAttribute("aria-hidden", "true");
    expect(copy.querySelector(".leave-panel")).not.toBeNull();
    // Opening's classes are off it, or its own animation would run again.
    expect(copy.classList.contains("settle-scrim")).toBe(false);
    expect(copy.querySelector(".sheet-in")).toBeNull();

    act(() => void vi.advanceTimersByTime(150));
    expect(ghost()).not.toBeNull();
    act(() => void vi.advanceTimersByTime(40));
    expect(ghost()).toBeNull();
  });

  it("leaves the same way when a destination is picked", () => {
    const { scrim } = open();
    fireEvent.click(within(scrim).getByRole("link", { name: /Bundles/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(ghost()).not.toBeNull();
  });

  it("leaves nothing behind in reduced motion: sheet and scrim go together", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    open();
    fireEvent(window, new PopStateEvent("popstate"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(ghost()).toBeNull();
  });
});

describe("the navigation sheet dragged by its grabber (S3)", () => {
  it("follows the finger 1:1 and the scrim thins with it", () => {
    const { scrim, panel, grabber } = open();
    drag(grabber, 150);
    expect(panel.style.transform).toBe("translateY(150px)");
    // 150 of 600 is a quarter: its own 70% wash is thinned by a quarter.
    expect(scrim.style.backgroundColor).toBe("rgba(0, 0, 0, 0.525)");
  });

  it("settles back when let go short of a third, and stays open", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const { scrim, panel, grabber } = open();
    drag(grabber, 150)();
    expect(panel.style.transform).toBe("");
    expect(scrim.style.backgroundColor).toBe("");
    act(() => void vi.advanceTimersByTime(600));
    expect(back).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("leaves past a third, on settle's duration, by spending its history entry, with no copy left", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const { panel, grabber } = open();
    drag(grabber, 250)();
    expect(panel.style.transform).toBe("translateY(100%)");
    expect(back).not.toHaveBeenCalled();
    act(() => void vi.advanceTimersByTime(260));
    expect(back).toHaveBeenCalledTimes(1);

    // The back gesture lands: the sheet has already left, so it leaves no picture of itself.
    fireEvent(window, new PopStateEvent("popstate"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(ghost()).toBeNull();
  });

  it("goes back once, however many ways it is dismissed while it leaves", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const { scrim, grabber } = open();
    fireEvent.click(grabber);
    fireEvent.click(grabber);
    fireEvent.keyDown(scrim, { key: "Escape" });
    // A second back would leave the screen behind the sheet.
    expect(back).toHaveBeenCalledTimes(1);
  });

  it("a drag is not a tap: the click that ends it does not close", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const { grabber } = open();
    drag(grabber, 20)();
    fireEvent.click(grabber);
    expect(back).not.toHaveBeenCalled();
    fireEvent.click(grabber);
    expect(back).toHaveBeenCalledTimes(1);
  });

  it("in reduced motion a drag does not move it, and past a third it closes at once", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const { panel, grabber } = open();
    drag(grabber, 250);
    expect(panel.style.transform).toBe("");
    fireEvent.pointerUp(grabber, { clientY: 350, pointerId: 1 });
    expect(back).toHaveBeenCalledTimes(1);
  });
});

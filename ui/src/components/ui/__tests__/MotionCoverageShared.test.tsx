// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Button } from "@/components/ui/Button";
import { FilterChip } from "@/components/ui/FilterChip";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { SelectionBar } from "@/components/ui/SelectionBar";
import { motionInTests } from "@/test-utils/motion";

/** Shared pieces that used to pop or snap: they now move by tint (FINAL §4). */

describe("SelectionBar", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  const bar = (count: number) => (
    <SelectionBar count={count} noun={["person", "people"]} onClear={() => {}}>
      <span>Do it</span>
    </SelectionBar>
  );

  it("tints in, then tints out showing what it last said, ignoring the pointer, then goes", () => {
    const { rerender } = render(bar(0));
    expect(screen.queryByRole("region", { name: "Selection" })).toBeNull();

    rerender(bar(3));
    const region = screen.getByRole("region", { name: "Selection" });
    expect(region).toHaveClass("tint-in");

    rerender(bar(0));
    const leaving = screen.getByRole("region", { hidden: true });
    expect(leaving).toHaveAttribute("aria-hidden", "true");
    expect(leaving).toHaveClass("tint-out", "pointer-events-none");
    expect(leaving).toHaveTextContent("3 people selected");

    act(() => void vi.advanceTimersByTime(121));
    expect(screen.queryByRole("region", { name: "Selection" })).toBeNull();
  });
});

describe("FilterChip", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  it("tints in, and its own clear tints it out before the filter is cleared", () => {
    const onClear = vi.fn();
    render(
      <FilterChip onClear={onClear} clearLabel="Clear it">
        Laser Lab
      </FilterChip>,
    );
    const chip = screen.getByText("Laser Lab").parentElement!;
    expect(chip).toHaveClass("tint-in");

    fireEvent.click(screen.getByRole("button", { name: "Clear it" }));
    expect(chip).toHaveClass("tint-out", "pointer-events-none");
    expect(onClear).not.toHaveBeenCalled();

    act(() => void vi.advanceTimersByTime(121));
    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it("clears at once when motion is off", () => {
    motionInTests(false);
    const onClear = vi.fn();
    render(
      <FilterChip onClear={onClear} clearLabel="Clear it">
        Laser Lab
      </FilterChip>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Clear it" }));
    expect(onClear).toHaveBeenCalledTimes(1);
  });
});

describe("NotOpenYet", () => {
  it("tints its contents in when it opens, and is only its contents when open from the start", () => {
    const view = (open: boolean) => (
      <NotOpenYet open={open} title="Not yet" why="Needs a step">
        <p>Live part</p>
      </NotOpenYet>
    );
    const { rerender, unmount } = render(view(false));
    rerender(view(true));
    expect(screen.getByText("Live part").parentElement).toHaveClass("tint-in");
    unmount();

    render(view(true));
    expect(screen.getByText("Live part").parentElement).not.toHaveClass("tint-in");
  });
});

describe("Button", () => {
  it("seats the pending dot out of the flow, so the label does not shift", () => {
    const { rerender } = render(<Button>Save</Button>);
    expect(screen.getByRole("button")).toHaveClass("control", "press-scale", "relative");
    rerender(<Button isPending>Save</Button>);
    const dot = screen.getByRole("button").querySelector(".breathe")!;
    expect(dot).toHaveClass("absolute");
  });
});

describe("Notice", () => {
  // The first paint is just there; a Notice that mounts after it opens its
  // space by Unfold (A-08 §5), in place and without a rise.
  it("opens by Unfold once the app has painted, and never rises", async () => {
    Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
    try {
      const { Notice } = await import("@/components/ui/Notice");
      await new Promise((resolve) => setTimeout(resolve, 80));
      render(
        <Notice tone="danger" title="Could not load">
          Nothing was changed.
        </Notice>,
      );
      const box = screen.getByRole("status");
      expect(box.closest(".unfold")).toHaveAttribute("data-phase", "opening");
      expect(box.className).not.toMatch(/tint-in|lift-in|arrive|popover-in/);
    } finally {
      delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
    }
  });
});

// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Crossfade } from "@/components/ui/Crossfade";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S4: words crossfade on tint, both layers in one cell, so nothing shifts. */

beforeEach(() => {
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => vi.useRealTimers());

describe("Crossfade", () => {
  it("shows the first thing plainly, with nothing animating", () => {
    render(<Crossfade id="a">Approve</Crossfade>);
    expect(screen.getByText("Approve")).not.toHaveClass("tint-in");
  });

  it("stacks old and new in one cell while it fades, then keeps only the new", () => {
    const { rerender, container } = render(<Crossfade id="a">Approve</Crossfade>);
    rerender(<Crossfade id="b">Approved</Crossfade>);

    const old = screen.getByText("Approve");
    expect(old).toHaveClass("tint-out");
    expect(old).toHaveAttribute("inert");
    expect(old).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByText("Approved")).toHaveClass("tint-in");
    expect(container.firstElementChild).toHaveClass("grid");

    act(() => void vi.advanceTimersByTime(120));
    expect(screen.queryByText("Approve")).not.toBeInTheDocument();
    expect(screen.getByText("Approved")).not.toHaveClass("tint-in");
  });

  it("only swaps when its id changes: new content under the same id is not a change of word", () => {
    const { rerender } = render(<Crossfade id="a">one</Crossfade>);
    rerender(<Crossfade id="a">two</Crossfade>);
    expect(screen.getByText("two")).toBeInTheDocument();
    expect(screen.queryByText("one")).not.toBeInTheDocument();
  });

  it("swaps at once under reduced motion", () => {
    motionInTests(false);
    const { rerender } = render(<Crossfade id="a">Approve</Crossfade>);
    rerender(<Crossfade id="b">Approved</Crossfade>);
    expect(screen.queryByText("Approve")).not.toBeInTheDocument();
    expect(screen.getByText("Approved")).toBeInTheDocument();
  });
});

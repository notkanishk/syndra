// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { StatusSeat } from "@/components/ui/StatusSeat";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S4: a status changing in place. The dot stays and changes colour; the words crossfade. */

beforeEach(() => {
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => vi.useRealTimers());

describe("StatusSeat", () => {
  it("changes the dot's colour where it sits and crossfades the words, with no bloom", () => {
    const { container, rerender } = render(<StatusSeat tone="warn" word="Waiting on a decision" />);
    const dot = container.querySelector("span[aria-hidden]")!;
    rerender(<StatusSeat tone="healthy" word="Approved" note="by Ada" />);

    expect(container.querySelector("span[aria-hidden]")).toBe(dot);
    expect(dot.className).toContain("bg-healthy");
    expect(dot.className).toContain("motion-tint");
    // This is a change someone else made, or a read: never a bloom.
    expect(container.querySelector(".bloom")).toBeNull();
    expect(screen.getByText("Waiting on a decision").closest(".tint-out")).not.toBeNull();
    expect(screen.getByText("Approved").closest(".tint-in")).not.toBeNull();

    act(() => void vi.advanceTimersByTime(120));
    expect(screen.queryByText("Waiting on a decision")).not.toBeInTheDocument();
    expect(screen.getByText("by Ada")).toBeInTheDocument();
  });
});

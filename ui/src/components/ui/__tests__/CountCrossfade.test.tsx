// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CountChip } from "@/components/ui/Badge";
import { CardHeader } from "@/components/ui/Card";
import { TINT_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

/** D-1 M2: a changed count crossfades in its seat; the first read is one layer; nothing ticks or travels. */
afterEach(() => vi.useRealTimers());

describe("a count changes in place", () => {
  it("crossfades a number that changes between two reads: both layers for one tint", () => {
    vi.useFakeTimers();
    motionInTests();
    const view = render(<CountChip n={3} />);
    expect(screen.getAllByText("3")).toHaveLength(1);
    expect(document.querySelector(".tint-out")).toBeNull();

    view.rerender(<CountChip n={4} />);
    expect(screen.getByText("3").closest(".tint-out")).not.toBeNull();
    expect(screen.getByText("4").closest(".tint-in")).not.toBeNull();

    act(() => void vi.advanceTimersByTime(TINT_MS));
    expect(screen.queryByText("3")).toBeNull();
    expect(screen.getByText("4")).toBeInTheDocument();
  });

  it("shows one layer on the first read, and when a failed read becomes a number", () => {
    motionInTests();
    const view = render(<CountChip n={null} />);
    view.rerender(<CountChip n={5} />);
    expect(screen.getAllByText("5")).toHaveLength(1);
    expect(document.querySelector(".tint-out")).toBeNull();
  });

  it("crossfades the card header's count the same way, and swaps at once when still", () => {
    motionInTests();
    const view = render(<CardHeader title="Waiting" count={1} />);
    view.rerender(<CardHeader title="Waiting" count={2} />);
    expect(screen.getByTestId("card-count").querySelector(".tint-out")).not.toBeNull();
    view.unmount();

    motionInTests(false);
    const still = render(<CardHeader title="Waiting" count={1} />);
    still.rerender(<CardHeader title="Waiting" count={2} />);
    expect(screen.getByTestId("card-count")).toHaveTextContent("2");
    expect(screen.queryByText("1")).toBeNull();
  });
});

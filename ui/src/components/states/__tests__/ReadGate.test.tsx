// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SKELETON_DELAY_MS } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { TINT_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

function gate(loading: boolean) {
  return (
    <ReadGate loading={loading} skeleton={<p aria-label="Loading your access">…</p>}>
      <p>You can use the laser cutter</p>
    </ReadGate>
  );
}

/** S7 for a card or a screen that is one read: late skeleton, fade, then the content arrives. */
describe("ReadGate", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  it("shows nothing for the first 200ms, then the skeleton, never the content", () => {
    render(gate(true));
    expect(screen.queryByLabelText("Loading your access")).not.toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS));
    expect(screen.getByLabelText("Loading your access")).toBeInTheDocument();
    expect(screen.queryByText(/laser cutter/)).not.toBeInTheDocument();
  });

  it("a read inside 200ms never shows a skeleton: the content is there", () => {
    const { rerender } = render(gate(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS - 1));
    rerender(gate(false));
    expect(screen.queryByLabelText("Loading your access")).not.toBeInTheDocument();
    expect(screen.getByText(/laser cutter/)).toBeInTheDocument();
  });

  it("a skeleton that was shown fades first, then the content arrives; they do not crossfade", () => {
    const { rerender } = render(gate(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS));
    rerender(gate(false));

    expect(screen.getByLabelText("Loading your access").parentElement).toHaveClass("tint-out");
    expect(screen.queryByText(/laser cutter/)).not.toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(TINT_MS + 1));
    expect(screen.queryByLabelText("Loading your access")).not.toBeInTheDocument();
    expect(screen.getByText(/laser cutter/)).toBeInTheDocument();
  });
});

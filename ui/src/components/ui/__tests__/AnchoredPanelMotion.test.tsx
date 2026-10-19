// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AnchoredPanel } from "@/components/ui/Anchored";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** A popover closes on tint, opacity only, then unmounts (A-05 §2.3). */
function Popover({ open }: { open: boolean }) {
  const anchor = createRef<HTMLButtonElement>();
  return (
    <>
      <button ref={anchor} type="button">
        Source
      </button>
      <AnchoredPanel anchor={anchor} open={open} width={200}>
        <p>Given by hand</p>
      </AnchoredPanel>
    </>
  );
}

beforeEach(() => {
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => vi.useRealTimers());

describe("AnchoredPanel closing", () => {
  it("fades out on tint without moving, ignores the pointer meanwhile, then unmounts", () => {
    const { rerender } = render(<Popover open />);
    expect(screen.getByText("Given by hand")).toBeInTheDocument();

    rerender(<Popover open={false} />);
    const panel = screen.getByText("Given by hand").parentElement!;
    expect(panel.style.animation).toContain("fade-out");
    expect(panel.style.animation).not.toContain("sink");
    expect(panel).toHaveClass("pointer-events-none");
    expect(panel).toHaveAttribute("aria-hidden", "true");

    act(() => void vi.advanceTimersByTime(119));
    expect(screen.queryByText("Given by hand")).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(2));
    expect(screen.queryByText("Given by hand")).toBeNull();
  });

  it("is gone at once in reduced motion", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(<Popover open />);
    rerender(<Popover open={false} />);
    expect(screen.queryByText("Given by hand")).toBeNull();
  });

  it("opens again during the fade without losing it", () => {
    const { rerender } = render(<Popover open />);
    rerender(<Popover open={false} />);
    rerender(<Popover open />);
    act(() => void vi.advanceTimersByTime(300));
    const panel = screen.getByText("Given by hand").parentElement!;
    expect(panel.style.animation).toBe("");
    expect(panel).not.toHaveAttribute("aria-hidden");
  });
});

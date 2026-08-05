// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FLASH_MS, useFlashOnChange } from "@/lib/useFlashOnChange";

function Probe({ count, ready = true }: { count: number; ready?: boolean }) {
  const changed = useFlashOnChange(count, ready);
  return (
    <span data-testid="row" className={changed ? "flash" : ""}>
      {count}
    </span>
  );
}

const row = () => screen.getByTestId("row");
const flashing = () => row().className === "flash";

/**
 * The hook drops the class for one frame before re-applying it, because CSS
 * will not replay an animation whose class never left the element. Every test
 * that expects a visible flash has to cross that frame.
 */
function settle() {
  act(() => void vi.advanceTimersToNextFrame());
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("useFlashOnChange", () => {
  it("stays quiet on first paint", () => {
    render(<Probe count={12} />);
    settle();
    // A page that flashes every number on arrival has said nothing about which
    // one moved, and has spent the signal at the moment it is worthless.
    expect(flashing()).toBe(false);
  });

  it("flashes once when the value changes under the operator", () => {
    const { rerender } = render(<Probe count={11} />);
    settle();
    expect(flashing()).toBe(false);

    act(() => rerender(<Probe count={12} />));
    settle();
    expect(flashing()).toBe(true);

    // Exactly as long as the animation, and no longer: a row left holding the
    // class sits in a finished animation doing nothing.
    act(() => void vi.advanceTimersByTime(FLASH_MS - 1));
    expect(flashing()).toBe(true);
    act(() => void vi.advanceTimersByTime(1));
    expect(flashing()).toBe(false);
  });

  it("ignores a poll that returns the same number", () => {
    const { rerender } = render(<Probe count={12} />);
    act(() => rerender(<Probe count={12} />));
    settle();
    // Twelve unexplained grants are not news twice. It watches the value, not
    // the fetch.
    expect(flashing()).toBe(false);
  });

  // The bug this catches: `setFlashing(true)` while already true is a no-op in
  // React, so the class never leaves the element and CSS does not replay. The
  // timer restarted and nothing else — the second change was never marked.
  it("replays rather than merely extending when a second change lands mid-flash", () => {
    const { rerender } = render(<Probe count={1} />);
    settle();

    act(() => rerender(<Probe count={2} />));
    settle();
    expect(flashing()).toBe(true);

    act(() => void vi.advanceTimersByTime(FLASH_MS - 100));
    act(() => rerender(<Probe count={3} />));

    // The class must come OFF for a frame. Without that drop the animation
    // keeps running out the first flash and the operator never sees the
    // second update land.
    expect(flashing(), "the class must clear so the animation can restart").toBe(false);

    settle();
    expect(flashing()).toBe(true);
    act(() => void vi.advanceTimersByTime(FLASH_MS - 1));
    expect(flashing(), "the second flash runs its own full duration").toBe(true);
    act(() => void vi.advanceTimersByTime(1));
    expect(flashing()).toBe(false);
  });

  // The bug this catches: a query backed by `placeholderData` hands out a
  // fabricated zero before its first payload. A real 12 landing over that zero
  // is an arrival, not a change, and flashing it means every nonzero badge in
  // the rail flashes on every page load.
  describe("a value that is not real yet", () => {
    it("adopts the first real payload without announcing it", () => {
      const { rerender } = render(<Probe count={0} ready={false} />);
      settle();

      act(() => rerender(<Probe count={12} ready />));
      settle();
      expect(flashing(), "the first real value is an arrival").toBe(false);
    });

    it("still marks a change once the values are real", () => {
      const { rerender } = render(<Probe count={0} ready={false} />);
      act(() => rerender(<Probe count={12} ready />));
      settle();
      expect(flashing()).toBe(false);

      act(() => rerender(<Probe count={13} ready />));
      settle();
      expect(flashing(), "a change between two real readings is news").toBe(true);
    });

    it("says nothing while the placeholder itself moves", () => {
      const { rerender } = render(<Probe count={0} ready={false} />);
      act(() => rerender(<Probe count={4} ready={false} />));
      settle();
      expect(flashing()).toBe(false);
    });
  });
});

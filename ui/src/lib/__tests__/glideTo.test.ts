// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { glideTo } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

/**
 * The one glide every jump to a section goes through (Motion 8d): the page
 * scrolls on settle's curve over the glide token, then the landing heading
 * flashes once. Still, it jumps and the flash stays (colour only).
 */
function page(scrollTop = 0) {
  const scroller = document.createElement("div");
  scroller.id = "app-scroll";
  scroller.scrollTo = vi.fn(({ top }: ScrollToOptions) => {
    scroller.scrollTop = top ?? 0;
  }) as unknown as typeof scroller.scrollTo;
  scroller.scrollTop = scrollTop;
  scroller.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const heading = document.createElement("h2");
  heading.getBoundingClientRect = () => ({ top: 900 - scroller.scrollTop }) as DOMRect;
  scroller.append(heading);
  document.body.append(scroller);
  return { scroller, heading };
}

afterEach(() => {
  document.body.innerHTML = "";
  document.documentElement.style.cssText = "";
  motionInTests(false);
  vi.useRealTimers();
});

describe("glideTo", () => {
  it("jumps in one step and still flashes when motion is still", () => {
    const { scroller, heading } = page();
    glideTo(heading);
    expect(scroller.scrollTo).toHaveBeenCalledTimes(1);
    // 900 down, less the 64 the heading is left clear of the top by; never smooth.
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 836 });
    expect(heading.classList.contains("flash")).toBe(true);
  });

  describe("with motion", () => {
    beforeEach(() => {
      motionInTests();
      vi.useFakeTimers();
      document.documentElement.style.setProperty("--duration-glide", "520ms");
      document.documentElement.style.setProperty("--ease-settle", "cubic-bezier(0.16, 1, 0.3, 1)");
    });

    it("moves a little each frame over the glide token, lands exactly, then flashes", () => {
      const { scroller, heading } = page();
      glideTo(heading);
      expect(heading.classList.contains("flash")).toBe(false);

      vi.advanceTimersByTime(260);
      const mid = scroller.scrollTop;
      expect(mid).toBeGreaterThan(0);
      expect(mid).toBeLessThan(836);
      expect(heading.classList.contains("flash")).toBe(false);

      vi.advanceTimersByTime(400);
      expect(scroller.scrollTop).toBe(836);
      expect(heading.classList.contains("flash")).toBe(true);
    });

    it("decelerates: most of the way is covered in the first half", () => {
      const { scroller, heading } = page();
      glideTo(heading);
      vi.advanceTimersByTime(260);
      expect(scroller.scrollTop).toBeGreaterThan(836 / 2);
    });

    it("a second jump starts over from wherever the page is", () => {
      const { scroller, heading } = page();
      glideTo(heading);
      vi.advanceTimersByTime(100);
      glideTo(heading);
      vi.advanceTimersByTime(700);
      expect(scroller.scrollTop).toBe(836);
    });
  });
});

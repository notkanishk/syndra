// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeToggle } from "@/components/shell/ThemeToggle";
import { ThemeProvider } from "@/lib/theme";
import { setMediaQuery } from "@/test-utils/media";

const REDUCE = "(prefers-reduced-motion: reduce)";
const html = document.documentElement;

describe("S14 Daylight: the theme switch crossfades every colour, once", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    setMediaQuery(REDUCE, false);
    html.classList.remove("theme-fading");
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  function toggle() {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Switch to/ }));
  }

  it("marks the page as fading for the switch, then takes the mark off", () => {
    toggle();
    expect(html.classList.contains("theme-fading")).toBe(true);
    expect(html.getAttribute("data-theme")).toBeTruthy();
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(html.classList.contains("theme-fading")).toBe(true);
    act(() => {
      vi.advanceTimersByTime(2);
    });
    expect(html.classList.contains("theme-fading")).toBe(false);
  });

  it("does not fade on first paint, only when the reader switches", () => {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>,
    );
    expect(html.classList.contains("theme-fading")).toBe(false);
  });

  it("is skipped under reduced motion", () => {
    setMediaQuery(REDUCE, true);
    toggle();
    expect(html.classList.contains("theme-fading")).toBe(false);
  });

  it("two quick switches stay faded until 300ms after the last one", () => {
    toggle();
    act(() => {
      vi.advanceTimersByTime(200);
    });
    fireEvent.click(screen.getByRole("button", { name: /Switch to/ }));
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(html.classList.contains("theme-fading")).toBe(true);
    act(() => {
      vi.advanceTimersByTime(101);
    });
    expect(html.classList.contains("theme-fading")).toBe(false);
  });

  it("the toggle's glyphs crossfade where they sit: both drawn, neither one turns or travels", () => {
    toggle();
    const ring = screen.getByRole("button", { name: /Switch to/ }).querySelector("[data-thumb]");
    expect(ring).not.toBeNull();
    expect(ring!.querySelectorAll("svg").length).toBe(2);
    expect(ring!.innerHTML).not.toMatch(/rotate|translate/);
  });
});

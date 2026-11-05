// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeToggle } from "@/components/shell/ThemeToggle";
import { ThemeProvider, useTheme } from "@/lib/theme";
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

// A-19 P11: "Device" is the no-stored-value path, chosen on purpose.
describe("Appearance: Device", () => {
  const DARK = "(prefers-color-scheme: dark)";
  const stored = () => localStorage.getItem("syndra-theme");

  function Probe() {
    const { choice, theme, setTheme } = useTheme();
    return (
      <>
        <output data-testid="state">{`${choice}/${theme}`}</output>
        <button onClick={() => setTheme("light")}>light</button>
        <button onClick={() => setTheme("dark")}>dark</button>
        <button onClick={() => setTheme("device")}>device</button>
      </>
    );
  }
  const state = () => screen.getByTestId("state").textContent;

  beforeEach(() => {
    localStorage.clear();
    setMediaQuery(REDUCE, false);
    setMediaQuery(DARK, false);
    html.classList.remove("theme-fading");
  });
  afterEach(() => cleanup());

  it("clears the stored theme and follows the OS, marking the page as system-driven", () => {
    setMediaQuery(DARK, true);
    localStorage.setItem("syndra-theme", "light");
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(state()).toBe("light/light");

    fireEvent.click(screen.getByText("device"));
    expect(stored()).toBeNull();
    expect(state()).toBe("device/dark");
    expect(html.getAttribute("data-theme")).toBe("dark");
    expect(html.getAttribute("data-theme-system")).toBe("dark");
  });

  it("keeps following the device after being chosen, and crossfades at the change", () => {
    localStorage.setItem("syndra-theme", "dark");
    render(<ThemeProvider><Probe /></ThemeProvider>);
    fireEvent.click(screen.getByText("device"));
    expect(state()).toBe("device/light");
    html.classList.remove("theme-fading");

    act(() => setMediaQuery(DARK, true));
    expect(state()).toBe("device/dark");
    expect(html.getAttribute("data-theme")).toBe("dark");
    expect(html.classList.contains("theme-fading")).toBe(true);
    expect(stored()).toBeNull();
  });

  it("stops following the device the moment Dark or Light is chosen", () => {
    render(<ThemeProvider><Probe /></ThemeProvider>);
    fireEvent.click(screen.getByText("light"));
    expect(stored()).toBe("light");
    expect(html.getAttribute("data-theme-system")).toBeNull();

    act(() => setMediaQuery(DARK, true));
    expect(state()).toBe("light/light");
  });

  it("opens as Device when nothing is stored, and as the stored choice otherwise", () => {
    const first = render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(state().startsWith("device/")).toBe(true);
    first.unmount();
    localStorage.setItem("syndra-theme", "dark");
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(state()).toBe("dark/dark");
  });
});

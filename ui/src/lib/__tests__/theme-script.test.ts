// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { THEME_SCRIPT } from "@/lib/theme-script";

function run({ stored, osDark }: { stored?: string; osDark: boolean }) {
  localStorage.clear();
  if (stored) localStorage.setItem("syndra-theme", stored);
  vi.stubGlobal("matchMedia", () => ({ matches: osDark }));
  document.documentElement.removeAttribute("data-theme");
  new Function(THEME_SCRIPT)();
  return document.documentElement.getAttribute("data-theme");
}

afterEach(() => vi.unstubAllGlobals());

describe("the pre-paint theme script", () => {
  it("follows the OS when nothing is stored, as ThemeProvider does", () => {
    expect(run({ osDark: false })).toBe("light");
    expect(run({ osDark: true })).toBe("dark");
  });

  it("lets a stored choice beat the OS", () => {
    expect(run({ stored: "light", osDark: true })).toBe("light");
    expect(run({ stored: "dark", osDark: false })).toBe("dark");
  });

  it("falls back to dark where nothing can be read", () => {
    localStorage.clear();
    vi.stubGlobal("matchMedia", undefined);
    new Function(THEME_SCRIPT)();
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });
});

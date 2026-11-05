"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

import { LIGHT_LEAD_MS, SETTLE_MS } from "@/lib/motion";

type Theme = "light" | "dark";
type Resolved = Theme | null;
/** What the reader asked for. "device" is the no-stored-value path: follow prefers-color-scheme. */
export type ThemeChoice = Theme | "device";

interface ThemeContextValue {
  theme: Resolved;
  /** The ask, as the Appearance control shows it. */
  choice: ThemeChoice;
  toggle: () => void;
  setTheme: (next: ThemeChoice) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = "syndra-theme";
const DARK_QUERY = "(prefers-color-scheme: dark)";

function readStored(): Theme | null {
  const v = typeof window !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
  return v === "light" || v === "dark" ? v : null;
}

function deviceTheme(): Theme {
  return typeof window !== "undefined" && window.matchMedia?.(DARK_QUERY).matches ? "dark" : "light";
}

/**
 * ThemeProvider toggles a `data-theme="light"|"dark"` attribute on
 * <html> and persists the user's choice to localStorage. "Device" is the
 * absence of a stored value: the OS preference is honored via
 * `prefers-color-scheme` mirrored into `data-theme-system`, and followed live
 * while it stays that way. SSR-safe: the initial render is `null` and the
 * effect populates the resolved theme on mount, avoiding a hydration mismatch.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Resolved>(null);
  const [choice, setChoice] = useState<ThemeChoice>("device");

  useEffect(() => {
    const stored = readStored();
    if (stored) {
      applyTheme(stored, false);
      setThemeState(stored);
      setChoice(stored);
    } else {
      const initial = deviceTheme();
      applyTheme(initial, true);
      setThemeState(initial);
    }
  }, []);

  // While the reader has asked for Device, mirror the OS and crossfade at a change (S14).
  useEffect(() => {
    if (choice !== "device") return;
    const media = window.matchMedia?.(DARK_QUERY);
    if (!media) return;
    const onChange = (e: MediaQueryListEvent) => {
      const next: Theme = e.matches ? "dark" : "light";
      fadeThemeSwitch();
      applyTheme(next, true);
      setThemeState(next);
    };
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [choice]);

  const setTheme = useCallback((next: ThemeChoice) => {
    const resolved = next === "device" ? deviceTheme() : next;
    if (next === "device") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, next);
    fadeThemeSwitch();
    applyTheme(resolved, next === "device");
    setThemeState(resolved);
    setChoice(next);
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  return (
    <ThemeContext.Provider value={{ theme, choice, toggle, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

/** S14 Daylight: how long html wears `theme-fading` after a switch (settle's 260ms, and a beat). */
const FADE_MS = SETTLE_MS + LIGHT_LEAD_MS;
let fadeTimer: ReturnType<typeof setTimeout> | undefined;

/**
 * Crossfade every colour for this one switch. The class turns a short
 * transition on (globals.css) for the duration and is then taken off, so no
 * other colour change in the product is slowed by it. A reader who has asked
 * for less motion gets the switch itself and no fade.
 */
export function fadeThemeSwitch() {
  if (typeof document === "undefined") return;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;
  const root = document.documentElement;
  root.classList.add("theme-fading");
  clearTimeout(fadeTimer);
  fadeTimer = setTimeout(() => root.classList.remove("theme-fading"), FADE_MS);
}

function applyTheme(theme: Theme, isSystem: boolean) {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-theme", theme);
  if (isSystem) {
    document.documentElement.setAttribute("data-theme-system", theme);
  } else {
    document.documentElement.removeAttribute("data-theme-system");
  }
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    // Permissive fallback for tests / SSR contexts that render outside the provider.
    return {
      theme: null,
      choice: "device",
      toggle: () => {},
      setTheme: () => {},
    };
  }
  return ctx;
}

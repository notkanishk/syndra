"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

type Theme = "light" | "dark";
type Resolved = Theme | null;

interface ThemeContextValue {
  theme: Resolved;
  toggle: () => void;
  setTheme: (next: Theme) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = "syndra-theme";

/**
 * ThemeProvider toggles a `data-theme="light"|"dark"` attribute on
 * <html> and persists the user's choice to localStorage. When no
 * explicit override is stored, the OS preference is honored via
 * `prefers-color-scheme` mirrored into `data-theme-system`. SSR-safe:
 * the initial render is `null` and the effect populates the resolved
 * theme on mount, avoiding a hydration mismatch.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Resolved>(null);

  useEffect(() => {
    const stored = (typeof window !== "undefined" && (localStorage.getItem(STORAGE_KEY) as Theme | null)) || null;
    const prefersDark = typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches;
    const initial: Theme = stored ?? (prefersDark ? "dark" : "light");
    applyTheme(initial, !stored);
    setThemeState(initial);

    if (stored) return;
    // Mirror system changes when the user hasn't picked an explicit theme.
    const media = window.matchMedia?.("(prefers-color-scheme: dark)");
    if (!media) return;
    const onChange = (e: MediaQueryListEvent) => {
      if (localStorage.getItem(STORAGE_KEY)) return;
      const next: Theme = e.matches ? "dark" : "light";
      applyTheme(next, true);
      setThemeState(next);
    };
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  const setTheme = useCallback((next: Theme) => {
    if (typeof window !== "undefined") {
      localStorage.setItem(STORAGE_KEY, next);
    }
    applyTheme(next, false);
    setThemeState(next);
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  return (
    <ThemeContext.Provider value={{ theme, toggle, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
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
      toggle: () => {},
      setTheme: () => {},
    };
  }
  return ctx;
}

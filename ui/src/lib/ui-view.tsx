"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import type { Audience, UiView } from "@/lib/nav";

/**
 * `ui_view` — which of the two surfaces the operator is looking at.
 *
 * Deliberately NOT called `mode`: GET /api/v1/system/mode already exists and
 * reports demo-vs-live backend state. Two different "modes" in one product is
 * how the previous information architecture drifted.
 *
 * Three rules the switch must keep:
 *   1. The URL never changes. Switching reveals panels in place.
 *   2. No dead ends — Basic offers a scoped jump into Advanced that lands on
 *      the same URL with the newly revealed panel scrolled into view.
 *   3. Basic is not Advanced-minus-features. It is a smaller job done whole.
 */

const STORAGE_KEY = "mkauth_ui_view";

interface UiViewContextValue {
  view: UiView;
  /** 'member' when the session is not an operator — no switch is rendered. */
  audience: Audience;
  isOperator: boolean;
  setView: (next: UiView) => void;
  /**
   * Reveal an Advanced-only panel without leaving the page: sets the view and
   * scrolls the named panel into view once it mounts.
   */
  revealInAdvanced: (panelId: string) => void;
}

const UiViewContext = createContext<UiViewContextValue | null>(null);

export function UiViewProvider({
  isOperator,
  children,
}: {
  isOperator: boolean;
  children: React.ReactNode;
}) {
  const [view, setViewState] = useState<UiView>("basic");
  const [pendingPanel, setPendingPanel] = useState<string | null>(null);

  // Read the persisted preference after mount: rendering it on the server
  // would mean a hydration mismatch on every operator whose stored view is
  // not the default.
  useEffect(() => {
    if (!isOperator) return;
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "basic" || stored === "advanced") setViewState(stored);
  }, [isOperator]);

  const setView = useCallback(
    (next: UiView) => {
      if (!isOperator) return;
      setViewState(next);
      window.localStorage.setItem(STORAGE_KEY, next);
    },
    [isOperator],
  );

  const revealInAdvanced = useCallback(
    (panelId: string) => {
      setView("advanced");
      setPendingPanel(panelId);
    },
    [setView],
  );

  // Scroll the revealed panel into view on the frame after it mounts. We
  // scroll the scrolling container rather than calling scrollIntoView on the
  // element, so the whole page never jumps sideways in a narrow viewport.
  useEffect(() => {
    if (!pendingPanel) return;
    const frame = requestAnimationFrame(() => {
      const target = document.getElementById(pendingPanel);
      const container = document.getElementById("app-scroll");
      if (target && container) {
        const offset = target.getBoundingClientRect().top - container.getBoundingClientRect().top;
        container.scrollTo({ top: container.scrollTop + offset - 24, behavior: "smooth" });
      }
      setPendingPanel(null);
    });
    return () => cancelAnimationFrame(frame);
  }, [pendingPanel, view]);

  const value = useMemo<UiViewContextValue>(
    () => ({
      view: isOperator ? view : "basic",
      audience: isOperator ? view : "member",
      isOperator,
      setView,
      revealInAdvanced,
    }),
    [isOperator, view, setView, revealInAdvanced],
  );

  return <UiViewContext.Provider value={value}>{children}</UiViewContext.Provider>;
}

export function useUiView(): UiViewContextValue {
  const ctx = useContext(UiViewContext);
  if (!ctx) {
    // Permissive fallback so components render in tests and in the
    // unauthenticated layout, which mounts no provider.
    return {
      view: "basic",
      audience: "basic",
      isOperator: false,
      setView: () => {},
      revealInAdvanced: () => {},
    };
  }
  return ctx;
}

/** True when the Advanced-only content on a shared page should be rendered. */
export function useIsAdvanced(): boolean {
  return useUiView().view === "advanced";
}

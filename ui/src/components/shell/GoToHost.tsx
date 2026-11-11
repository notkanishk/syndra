"use client";

import { useEffect, useState } from "react";

import { GoTo } from "@/components/ui/GoTo";
import { otherDialogOpen, typingIn } from "@/lib/keys";
import { memberNav, navFor, targetNav } from "@/lib/nav";
import { useMyStorage } from "@/lib/queries/useMyStorage";
import { useTargets } from "@/lib/queries/useTargets";
import { useUiView } from "@/lib/ui-view";

/** Dispatch on `window` to open Go to by touch (the phone top bar's button). */
export const OPEN_GO_TO = "syndra:open-go-to";

/**
 * Mounts Go to once and owns its two keys: `/` when no field has the focus, and
 * Cmd K (Ctrl K elsewhere). It never fires while somebody is typing, and it
 * never opens on top of another dialog (sheets push, they do not stack).
 */
export function GoToHost() {
  const [open, setOpen] = useState(false);
  const { audience, isOperator } = useUiView();
  const { data: targets } = useTargets(isOperator);
  const { data: mine } = useMyStorage(audience === "member");
  const nav =
    audience === "advanced"
      ? targetNav(targets ?? [])
      : audience === "member"
        ? memberNav(mine ?? [])
        : navFor(audience);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.defaultPrevented) return;
      const chord = (event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === "k";
      const slash = event.key === "/" && !event.metaKey && !event.ctrlKey && !event.altKey && !typingIn(event.target);
      if (!chord && !slash) return;
      // Another dialog is open (Go to's own is told apart by its label).
      if (otherDialogOpen('[role="combobox"][aria-label="Go to"]')) return;
      event.preventDefault();
      setOpen((was) => (chord ? !was : true));
    }
    // The phone's top bar button (A-20 P6) asks for the same dialog by event, so
    // there is one Go to and one place that decides it may open.
    function onAsk() {
      if (otherDialogOpen('[role="combobox"][aria-label="Go to"]')) return;
      setOpen(true);
    }
    window.addEventListener("keydown", onKey);
    window.addEventListener(OPEN_GO_TO, onAsk);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener(OPEN_GO_TO, onAsk);
    };
  }, []);

  return <GoTo open={open} onClose={() => setOpen(false)} nav={nav} />;
}

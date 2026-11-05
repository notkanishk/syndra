"use client";

import { useEffect, useState } from "react";

import { GoTo } from "@/components/ui/GoTo";
import { otherDialogOpen, typingIn } from "@/lib/keys";
import { navFor, targetNav } from "@/lib/nav";
import { useTargets } from "@/lib/queries/useTargets";
import { useUiView } from "@/lib/ui-view";

/**
 * Mounts Go to once and owns its two keys: `/` when no field has the focus, and
 * Cmd K (Ctrl K elsewhere). It never fires while somebody is typing, and it
 * never opens on top of another dialog (sheets push, they do not stack).
 */
export function GoToHost() {
  const [open, setOpen] = useState(false);
  const { audience, isOperator } = useUiView();
  const { data: targets } = useTargets(isOperator);
  const nav = audience === "advanced" ? targetNav((targets ?? []).map((t) => t.target)) : navFor(audience);

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
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return <GoTo open={open} onClose={() => setOpen(false)} nav={nav} />;
}

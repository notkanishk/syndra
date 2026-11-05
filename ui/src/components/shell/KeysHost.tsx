"use client";

import { useEffect, useState } from "react";

import { KeysSheet } from "@/components/ui/KeysSheet";
import { otherDialogOpen, typingIn } from "@/lib/keys";

/**
 * Mounts the Keys sheet once and owns its key: `?` when no field has the focus
 * and nothing else is open. Esc is the dialog's own. A page that answers `?`
 * itself (Insights' map) says so by taking the event first.
 */
export function KeysHost() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.defaultPrevented || event.key !== "?" || event.metaKey || event.ctrlKey || event.altKey) return;
      if (typingIn(event.target) || otherDialogOpen("[data-keys-sheet]")) return;
      event.preventDefault();
      setOpen(true);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return <KeysSheet open={open} onClose={() => setOpen(false)} />;
}

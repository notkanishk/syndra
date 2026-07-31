"use client";

import { useEffect, useState } from "react";

import { CHIME_FIRST_PLAY_EVENT, isChimeEnabled, setChimeEnabled } from "@/lib/driftChime";

/**
 * The unexplained-access chime.
 *
 * It used to live in the sidebar footer, next to an org-wide policy control —
 * a browser preference and a policy sitting in the same box is how somebody
 * changes the wrong one. It now lives in Automation settings, labelled as the
 * per-browser preference it is.
 */
export function ChimeToggle() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [heard, setHeard] = useState(false);

  useEffect(() => {
    setEnabled(isChimeEnabled());
    const onFirstPlay = () => setHeard(true);
    window.addEventListener(CHIME_FIRST_PLAY_EVENT, onFirstPlay);
    return () => window.removeEventListener(CHIME_FIRST_PLAY_EVENT, onFirstPlay);
  }, []);

  // Render nothing until the stored preference is read: a toggle that flips
  // itself after hydration reads as the setting having changed.
  if (enabled === null) return <span aria-hidden className="h-8 w-[104px]" />;

  return (
    <span className="flex flex-col items-end gap-1.5">
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        onClick={() => {
          setChimeEnabled(!enabled);
          setEnabled(!enabled);
        }}
        className={`rounded-pill px-4 py-[7px] text-[13.5px] font-semibold transition-colors ${
          enabled ? "bg-accent text-accent-ink" : "border border-line-strong text-muted"
        }`}
      >
        {enabled ? "Sound on" : "Sound off"}
      </button>
      {heard && <span className="text-[12.5px] text-faint">That was this.</span>}
    </span>
  );
}

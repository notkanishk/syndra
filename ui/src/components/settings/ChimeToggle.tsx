"use client";

import { useEffect, useState } from "react";

import { PILL } from "@/components/ui/Button";

import { CHIME_FIRST_PLAY_EVENT, isChimeEnabled, setChimeEnabled } from "@/lib/driftChime";
import { useReducedMotion } from "@/lib/useViewport";

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
  // The chime is silenced for anybody who has asked for less motion — a sound
  // that arrives unbidden is movement in the room. That is correct, and it
  // means this control can otherwise say "Sound on" while guaranteeing
  // silence, which is a lie the page is in a position to catch.
  const quieted = useReducedMotion();

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
        aria-label="Sound on new unexplained access"
        onClick={() => {
          setChimeEnabled(!enabled);
          setEnabled(!enabled);
        }}
        className={`rounded-pill font-semibold motion-tint ${PILL.md} ${
          enabled ? "bg-accent-dense text-accent-ink" : "border border-line-strong text-muted"
        }`}
      >
        {enabled ? "Sound on" : "Sound off"}
      </button>
      {enabled && quieted && (
        <span className="text-[12.5px] leading-[1.5] text-faint">
          Nothing will play: this browser is set to reduce motion, and a sound arriving on its
          own is movement.
        </span>
      )}
      {heard && <span className="text-[12.5px] text-faint">That was this.</span>}
    </span>
  );
}

"use client";

import { useEffect, useState } from "react";

import { CHIME_FIRST_PLAY_EVENT, isChimeEnabled, setChimeEnabled } from "@/lib/driftChime";

/**
 * Bell icon toggle for the drift chime, mounted beside ThemeToggle. Renders
 * nothing until the stored preference is read client-side (avoids a
 * hydration mismatch), same pattern as ThemeToggle's null-theme guard.
 *
 * Also owns the one-time "what was that sound" tooltip: driftChime fires
 * CHIME_FIRST_PLAY_EVENT the first time it actually plays, and this is the
 * one place in the tree that's always mounted (sidebar) to catch it.
 */
export default function ChimeToggle() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [showTooltip, setShowTooltip] = useState(false);

  useEffect(() => {
    setEnabled(isChimeEnabled());
    function onFirstPlay() {
      setShowTooltip(true);
    }
    window.addEventListener(CHIME_FIRST_PLAY_EVENT, onFirstPlay);
    return () => window.removeEventListener(CHIME_FIRST_PLAY_EVENT, onFirstPlay);
  }, []);

  if (enabled === null) {
    return <div aria-hidden="true" className="h-8 w-8" />;
  }

  function toggle() {
    const next = !enabled;
    setChimeEnabled(next);
    setEnabled(next);
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={toggle}
        aria-label={enabled ? "Mute drift chime" : "Unmute drift chime"}
        title={enabled ? "Mute drift chime" : "Unmute drift chime"}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-colors hover:text-on-surface hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        <svg
          viewBox="0 0 24 24"
          width="16"
          height="16"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
          {!enabled && <path d="M3 3l18 18" />}
        </svg>
      </button>
      {showTooltip && (
        <div
          role="tooltip"
          className="absolute right-0 top-full z-50 mt-2 w-56 rounded-card border border-outline-variant bg-surface-container-low p-3 text-xs text-on-surface shadow-lg"
        >
          <p>A chime plays when new drift is detected — toggle it off here anytime.</p>
          <button
            type="button"
            onClick={() => setShowTooltip(false)}
            className="mt-2 font-medium text-primary hover:underline"
          >
            Got it
          </button>
        </div>
      )}
    </div>
  );
}

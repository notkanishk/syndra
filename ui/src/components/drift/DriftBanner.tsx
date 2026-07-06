"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { playDriftChime } from "@/lib/driftChime";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";

/**
 * Sticky, UNDISMISSIBLE top banner shown on every admin page while drift
 * exists. Red (error tokens), breaks out of the normal in-layout flow —
 * deliberately louder than the amber, dismissible PendingCallout. Slide-in
 * motion is suppressed under prefers-reduced-motion. Also owns the drift
 * chime cue: fires once whenever the count rises (new drift arrived), gated
 * on the chime toggle + prefers-reduced-motion inside playDriftChime itself.
 */
export function DriftBanner() {
  const { data } = useGovernanceSummary();
  const count = data?.drift?.count ?? 0;
  const prevCount = useRef(count);
  const [countRose, setCountRose] = useState(false);

  useEffect(() => {
    if (count > prevCount.current) {
      playDriftChime();
      setCountRose(true);
    }
    prevCount.current = count;
  }, [count]);

  if (count <= 0) return null;
  return (
    <div
      role="alert"
      className="sticky top-0 z-50 flex items-center justify-between gap-4 border-b border-error/50 bg-[color-mix(in_srgb,var(--error)_20%,transparent)] px-6 py-2 text-sm text-on-surface motion-safe:animate-slide-in-down"
    >
      <span>
        <span aria-hidden>⚠ </span>
        <strong
          className={countRose ? "inline-block motion-safe:animate-count-emphasis" : "inline-block"}
          onAnimationEnd={() => setCountRose(false)}
        >
          {count}
        </strong>{" "}
        drift {count === 1 ? "item" : "items"} detected — out-of-band changes need triage
      </span>
      <Link href="/governance/drift" className="font-medium text-error hover:underline">
        Review →
      </Link>
    </div>
  );
}

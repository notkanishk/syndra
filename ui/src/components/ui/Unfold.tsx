"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

type Phase = "opening" | "open" | "closing";

// A browser without the Web Animations API (jsdom, under test) would never
// report the end of a close, and the region would never leave. There it opens
// and closes at once.
const animates = () => typeof Element !== "undefined" && "getAnimations" in Element.prototype;

/**
 * A region that opens to its own height and pushes the page down, and on
 * closing fades first, then closes the gap (Motion 6a, 8a). The one way a
 * Notice, a fold or a disclosure enters and leaves, so nothing on the page
 * jumps when something above it appears.
 *
 * Present from the first paint, it is simply there: the page arriving is not
 * the region arriving. Its children stay mounted until the close has played,
 * so what leaves is still readable while it goes.
 */
export function Unfold({
  open,
  children,
  className = "",
  onClosed,
}: {
  open: boolean;
  children: ReactNode;
  className?: string;
  /** Called once the region is closed and gone (its gap shut), for what follows it. */
  onClosed?: () => void;
}) {
  const [phase, setPhase] = useState<Phase | null>(open ? "open" : null);
  // What leaves is what was there: the caller's children may already
  // describe the state that closed it.
  const shown = useRef(children);
  if (open) shown.current = children;
  const [was, setWas] = useState(open);

  // Adjusted during render, not in an effect, so the first frame of an
  // opening is already the collapsed one and nothing flashes at full height.
  if (was !== open) {
    setWas(open);
    const still = !animates();
    setPhase(open ? (still ? "open" : "opening") : phase === null || still ? null : "closing");
  }

  useEffect(() => {
    if (phase === null && !open) onClosed?.();
  }, [phase, open, onClosed]);

  if (phase === null) return null;

  return (
    <div
      className={`unfold ${className}`}
      data-phase={phase}
      onAnimationEnd={(e) => {
        if (e.target !== e.currentTarget) return;
        setPhase((p) => (p === "closing" ? null : p === "opening" ? "open" : p));
      }}
    >
      <div>{open ? children : shown.current}</div>
    </div>
  );
}

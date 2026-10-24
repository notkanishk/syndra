"use client";

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";

import { SETTLE_MS, TINT_MS, useExitPhase } from "@/lib/motion";

type Phase = "opening" | "open" | "closing";

/**
 * True inside an `Unfold` while it is opening. A part that opens itself when it
 * arrives late (a Notice) asks, so a caller that wraps it to give it a way out
 * does not get two openings, one inside the other. Only while opening: a part
 * that mounts later inside a region long open (a Home block) still opens itself.
 */
const UnfoldOpening = createContext(false);

// A browser without the Web Animations API (jsdom, under test) would never
// report the end of a close, and the region would never leave. There it opens
// and closes at once.
//
// Reduced motion is the same still form: the part appears and goes at once,
// rather than running a 0.01ms animation and waiting on its end.
const animates = () =>
  typeof Element !== "undefined" &&
  "getAnimations" in Element.prototype &&
  !window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches;

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
  gap,
  onClosed,
}: {
  open: boolean;
  children: ReactNode;
  className?: string;
  /**
   * The flex or grid gap (px) the parent puts around this region. The gap
   * beside a part that folds is part of what folds: it is drawn inside the
   * region and handed back by a negative margin, so the page does not hop by
   * one gap when the part arrives or leaves.
   */
  gap?: number;
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

  // The end of the move says it is over. Where no animation runs to say so (a
  // region inside a rail hidden on a phone), it is over after the move's own
  // length anyway, rather than left half open to play later in front of the reader.
  useEffect(() => {
    if (phase !== "opening" && phase !== "closing") return;
    const timer = setTimeout(
      () => setPhase((p) => (p === "closing" ? null : p === "opening" ? "open" : p)),
      TINT_MS + SETTLE_MS + 100,
    );
    return () => clearTimeout(timer);
  }, [phase]);

  if (phase === null) return null;

  return (
    <div
      className={`unfold ${className}`}
      style={gap ? { marginBottom: -gap } : undefined}
      data-phase={phase}
      onAnimationEnd={(e) => {
        // Each move ends on its last part: a close on the fold of the gap, an
        // opening on the fade of its contents, which runs one tint past the
        // height. Ended on the height, the fade would be cut off and the words
        // jump to full strength.
        const gap = e.target === e.currentTarget;
        const contents = e.target === e.currentTarget.firstElementChild;
        setPhase((p) => (p === "closing" && gap ? null : p === "opening" && contents ? "open" : p));
      }}
    >
      <div>
        <UnfoldOpening.Provider value={phase === "opening"}>
          {gap ? (
            <div style={{ paddingBottom: gap }}>{open ? children : shown.current}</div>
          ) : open ? (
            children
          ) : (
            shown.current
          )}
        </UnfoldOpening.Provider>
      </div>
    </div>
  );
}

/**
 * An inline bit of an Advanced line (a mono id inside a sentence): not a
 * region, so it has no height to open. It tints in when the view reaches it
 * and fades before it goes (A-08 §3), and one that is there at first paint is
 * simply there. Block-level parts use `Unfold`.
 */
export function Tinted({ show, children }: { show: boolean; children: ReactNode }) {
  const { rendered, leaving } = useExitPhase(show, TINT_MS);
  const [was, setWas] = useState(show);
  const [arrived, setArrived] = useState(false);
  if (was !== show) {
    setWas(show);
    if (show) setArrived(true);
  }
  if (!rendered) return null;
  return <span className={leaving ? "tint-out pointer-events-none" : arrived ? "tint-in" : undefined}>{children}</span>;
}

/**
 * Mounts closed and opens on the next effect: a part that arrives after first
 * paint opens by `Unfold` rather than appearing at its full height.
 */
export function UnfoldOnArrival({ children }: { children: ReactNode }) {
  // Where nothing animates it is simply there, in the first render.
  const [open, setOpen] = useState(!animates());
  useEffect(() => setOpen(true), []);
  return <Unfold open={open}>{children}</Unfold>;
}

export function useUnfoldOpening(): boolean {
  return useContext(UnfoldOpening);
}

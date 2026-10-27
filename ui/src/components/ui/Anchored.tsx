"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useState,
  type CSSProperties,
  type HTMLAttributes,
  type ReactNode,
  type RefObject,
} from "react";

import { ToBody } from "@/components/ui/Modal";
import { TINT_MS, useExitPhase } from "@/lib/motion";

/**
 * A popover drawn under <body>, at its trigger.
 *
 * Absolute inside the page, a popover is clipped by the first `overflow-hidden`
 * card or scrolling dialog around it: a source opened on a table's last row lost
 * its bottom half, one near the right edge its side. Portalled, nothing clips
 * it, so it is placed by hand: under the trigger, nudged inside the window, and
 * above it when there is more room up than down.
 *
 * `width` is in pixels, or "anchor" to match the trigger (a search box's
 * results). `expectHeight` is roughly how tall it opens, which decides the flip.
 */
export function useAnchoredBox(
  anchor: RefObject<HTMLElement | null>,
  open: boolean,
  width: number | "anchor",
  expectHeight: number,
  align: "start" | "end" = "start",
) {
  const [box, setBox] = useState<{ top: number; left: number; width: number; above: boolean } | null>(null);

  const place = useCallback(() => {
    const trigger = anchor.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const w = Math.min(width === "anchor" ? rect.width : width, window.innerWidth - 32);
    // "end" lines its right edge up with the trigger's, for a button at the
    // end of its row.
    const want = align === "end" ? rect.right - w : rect.left;
    const left = Math.max(16, Math.min(want, window.innerWidth - w - 16));
    const below = window.innerHeight - rect.bottom;
    const above = below < expectHeight && rect.top > below;
    setBox({ top: above ? rect.top - 7 : rect.bottom + 7, left, width: w, above });
  }, [anchor, width, expectHeight, align]);

  useLayoutEffect(() => {
    if (!open) return;
    place();
    // Recomputed while open: the page behind an open popover can still scroll.
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [open, place]);

  return box;
}

/** Fixed, at the box; hidden until measured so it never flashes at 0,0. */
export function anchoredStyle(box: ReturnType<typeof useAnchoredBox>): CSSProperties {
  if (!box) return { position: "fixed", visibility: "hidden" };
  return {
    position: "fixed",
    left: box.left,
    width: box.width,
    // Opening upward, it is placed by its bottom edge so its height needs no
    // measuring.
    ...(box.above ? { bottom: window.innerHeight - box.top } : { top: box.top }),
    // Over the page, and over a dialog's z-50: a popover opened inside a
    // dialog must show over it.
    zIndex: 60,
  };
}

export function AnchoredPanel({
  anchor,
  open,
  width,
  expectHeight = 240,
  align = "start",
  panelRef,
  id,
  className = "",
  panelProps,
  focusOnOpen = false,
  children,
}: {
  anchor: RefObject<HTMLElement | null>;
  open: boolean;
  width: number | "anchor";
  expectHeight?: number;
  align?: "start" | "end";
  id?: string;
  /** For an outside-click check: the panel is no longer inside the trigger. */
  panelRef?: RefObject<HTMLDivElement | null>;
  className?: string;
  /** Role, label and tabIndex for a panel that is a dialog rather than a list. */
  panelProps?: HTMLAttributes<HTMLDivElement>;
  /** Move focus onto the panel once it is placed. Needs `panelRef`. */
  focusOnOpen?: boolean;
  children: ReactNode;
}) {
  const box = useAnchoredBox(anchor, open, width, expectHeight, align);
  // Placed, not merely mounted: until measured it is `visibility: hidden`, and
  // a hidden element refuses focus.
  const placed = box !== null;
  useEffect(() => {
    if (focusOnOpen && open && placed) panelRef?.current?.focus({ preventScroll: true });
  }, [focusOnOpen, open, placed, panelRef]);
  // Closing is quicker than opening and has no travel: opacity on tint, then
  // gone. Opening's rise (4px, settle) is the caller's `popover-in`.
  const { rendered, leaving } = useExitPhase(open, TINT_MS);
  // It often closes because its content went (a suggestion list that emptied):
  // what fades is what it was showing, not an empty box.
  const [shown, setShown] = useState(children);
  if (open && shown !== children) setShown(children);
  if (!rendered) return null;
  return (
    <ToBody>
      <div
        {...panelProps}
        ref={panelRef}
        id={id}
        style={anchoredStyle(box)}
        className={leaving ? `${className} tint-out pointer-events-none` : className}
        aria-hidden={leaving ? true : undefined}
      >
        {leaving ? shown : children}
      </div>
    </ToBody>
  );
}

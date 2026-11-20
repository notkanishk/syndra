"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import { stillMotion } from "./still";

/**
 * The one indicator behind Segmented, FilterPills and Tabs (S10): a fill or an
 * underline that glides to the choice on settle, while the labels tint. It is
 * measured from the active button rather than computed from the options, so a
 * label that wraps, a count that widens and a font that loads all land it in
 * the right place. A control's own indicator is outside the travel cap.
 *
 * Put `ref` on the group, `relative`, and a `relative` class on each button so
 * it paints above the indicator; draw `<GlideIndicator>` first inside it. Until
 * the first measurement (server render, first paint) there is no box and the
 * caller gives the active button its own fill, so nothing is ever unmarked.
 */
const ACTIVE = '[aria-checked="true"], [aria-current="page"], [aria-current="location"]';

/** Where the active option is, in its group, and its own corners (for an indicator shaped as it is). */
export type Box = { left: number; top: number; width: number; height: number; radius: string };

const useIsoLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

export function useGlideIndicator<T extends HTMLElement = HTMLDivElement>(choice: unknown) {
  const ref = useRef<T>(null);
  const [box, setBox] = useState<Box | null>(null);

  const measure = useCallback(() => {
    const group = ref.current;
    const active = group?.querySelector<HTMLElement>(ACTIVE);
    // A choice inside a fold of the group's own that is moving (or shut) has no place yet: no indicator,
    // and the caller's own fill marks it, so a row folding away never leaves the indicator on the row that
    // follows. A fold around the whole group moves the group, not the choice within it, and its phase is
    // outside what this hook watches: a Segmented in an opening fold is measured as it is.
    const fold = active?.closest('.unfold:not([data-phase="open"])');
    if (!group || !active || (fold && group.contains(fold))) return setBox(null);
    const next = {
      left: active.offsetLeft,
      top: active.offsetTop,
      width: active.offsetWidth,
      height: active.offsetHeight,
      // One corner, the longhand: the shorthand reads "" in some engines. No corner read, a pill's.
      radius: getComputedStyle(active).borderTopLeftRadius || "var(--radius-pill)",
    };
    setBox((cur) =>
      cur && cur.left === next.left && cur.top === next.top && cur.width === next.width && cur.height === next.height && cur.radius === next.radius
        ? cur
        : next,
    );
  }, []);

  // When the choice changes; size changes are the observer's.
  useIsoLayoutEffect(measure, [measure, choice]);

  useEffect(() => {
    const group = ref.current;
    if (!group || typeof ResizeObserver === "undefined") return;
    const watch = new ResizeObserver(measure);
    watch.observe(group);
    // The segments, whatever their tag (a button, a link); not the indicator itself.
    group.querySelectorAll(":scope > :not([data-indicator]), :scope button, :scope a").forEach((segment) => watch.observe(segment));
    // A fold that opens or shuts, and a segment that arrives or leaves, moves the choice without
    // resizing anything the observer above was given (a row mounted after it, a fold settling open).
    const changes = new MutationObserver(measure);
    changes.observe(group, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-phase"] });
    return () => {
      watch.disconnect();
      changes.disconnect();
    };
  }, [measure]);

  return { ref, box };
}

export function GlideIndicator({
  box,
  kind,
  travel = true,
  className = "",
}: {
  box: Box | null;
  /** A fill shaped as the option is (its own corners: a pill's round ends, a rail row's 12px), or a
   *  line under it. */
  kind: "fill" | "underline";
  /** Whether a change of place glides; false lands it at once (a key, Back, a first paint). */
  travel?: boolean;
  /** The indicator's colour, a token class. */
  className?: string;
}) {
  if (!box) return null;
  const { left, top, width, height } =
    kind === "underline" ? { left: box.left, top: box.top + box.height - 1, width: box.width, height: 2 } : box;
  // One element, shaped as the active option at every point of the glide: its corners are the
  // option's own (round ends for a pill, 12px for a rail row; A-25 moment 4). It moves by transform
  // and takes the option's width and height: never scaled, so its ends never squash, and never drawn
  // in parts, whose overlaps doubled a translucent fill into two lines (staging, 20 Nov 2026). It is
  // out of flow, so a change of its size lays out itself alone, never the group.
  // Absent under reduced motion, and when the caller says this change does not travel: it lands at once.
  const glide = travel && !stillMotion() ? "indicator-glide" : "";
  return (
    <span
      aria-hidden
      data-indicator={kind}
      className={`pointer-events-none absolute left-0 top-0 ${kind === "underline" ? "rounded-pill" : ""} ${glide} ${className}`}
      style={{ transform: `translate(${left}px, ${top}px)`, width, height, borderRadius: kind === "fill" ? box.radius : undefined }}
    />
  );
}

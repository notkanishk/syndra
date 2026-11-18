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

type Box = { left: number; top: number; width: number; height: number };

const useIsoLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

export function useGlideIndicator(choice: unknown) {
  const ref = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState<Box | null>(null);

  const measure = useCallback(() => {
    const active = ref.current?.querySelector<HTMLElement>(ACTIVE);
    if (!active) return setBox(null);
    const next = { left: active.offsetLeft, top: active.offsetTop, width: active.offsetWidth, height: active.offsetHeight };
    setBox((cur) => (cur && cur.left === next.left && cur.top === next.top && cur.width === next.width && cur.height === next.height ? cur : next));
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
    return () => watch.disconnect();
  }, [measure]);

  return { ref, box };
}

export function GlideIndicator({
  box,
  kind,
  className = "",
}: {
  box: Box | null;
  kind: "fill" | "underline";
  /** The indicator's colour, a token class. */
  className?: string;
}) {
  if (!box) return null;
  const { left, top, width, height } =
    kind === "fill" ? box : { left: box.left, top: box.top + box.height - 1, width: box.width, height: 2 };
  // It moves by transform alone, so a glide is composited and never lays the
  // group out again (CSS Transforms 1; the compositor's one cheap property with
  // opacity). A pill scaled whole would squash its rounded ends, so it is drawn
  // as three parts: two caps, translated and never scaled, so they stay round,
  // and a square-ended middle, translated and scaled along x only. The parts
  // abut, never overlap, so a translucent fill is one even colour.
  const cap = Math.min(height, width) / 2;
  // Absent under reduced motion, where it jumps.
  const glide = stillMotion() ? "" : "indicator-glide";
  const part = `absolute left-0 top-0 ${glide} ${className}`;
  return (
    <span
      aria-hidden
      data-indicator
      className={`pointer-events-none absolute left-0 top-0 ${glide}`}
      style={{ transform: `translate(${left}px, ${top}px)` }}
    >
      <span className={`${part} rounded-l-pill`} style={{ width: cap, height }} />
      <span
        className={`${part} origin-left`}
        style={{ width: 1, height, transform: `translateX(${cap}px) scaleX(${Math.max(width - 2 * cap, 0)})` }}
      />
      <span className={`${part} rounded-r-pill`} style={{ width: cap, height, transform: `translateX(${width - cap}px)` }} />
    </span>
  );
}

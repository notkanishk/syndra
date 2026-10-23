"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import { stillMotion } from "@/lib/motion";

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
const ACTIVE = '[aria-checked="true"], [aria-current="page"]';

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
    group.querySelectorAll("button").forEach((button) => watch.observe(button));
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
  const place =
    kind === "fill"
      ? { left: box.left, top: box.top, width: box.width, height: box.height }
      : { left: box.left, top: box.top + box.height - 1, width: box.width, height: 2 };
  return (
    <span
      aria-hidden
      data-indicator
      // Absent under reduced motion, where it jumps.
      className={`pointer-events-none absolute rounded-pill ${stillMotion() ? "" : "indicator-glide"} ${className}`}
      style={place}
    />
  );
}

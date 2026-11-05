"use client";

import { useEffect, useRef, useState } from "react";

/**
 * The mark: a contained orb, and a miniature of the login's arch-and-orb.
 *
 * Three spans and no image — the geometry, the two masks and every colour
 * live in `globals.css` under `.syndra-mark`, so the rail and the favicon are
 * the same drawing rather than two that have to be kept in agreement, and the
 * light theme inverts it without this file knowing.
 *
 * `size` sets `--mark-size`; everything inside scales off it.
 *
 * `welcome` lights the dot once, at the session's first paint (A-18 idea 6):
 * the login's orb carried on, because you came through the door. The tab
 * remembers it has done so, so navigating or remounting never replays it, and
 * reduced motion removes the bloom itself (`bloom` in globals.css).
 */
export const WELCOMED = "syndra:mark-welcomed";

export function SyndraMark({
  size = 22,
  className = "",
  welcome = false,
}: {
  size?: number;
  className?: string;
  welcome?: boolean;
}) {
  const [bloom, setBloom] = useState(false);
  const seat = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    if (!welcome) return;
    // The rail's mark and the phone bar's both mount; only the one on screen
    // may spend the one-shot (checkVisibility is absent in old engines: then
    // the first to mount wins, as before).
    if (seat.current?.checkVisibility?.() === false) return;
    try {
      if (window.sessionStorage.getItem(WELCOMED)) return;
      window.sessionStorage.setItem(WELCOMED, "1");
      setBloom(true);
    } catch {
      // Storage blocked: no memory of the first paint, so no bloom rather than one on every mount.
    }
  }, [welcome]);
  return (
    <span
      ref={seat}
      aria-hidden
      className={`syndra-mark ${className}`}
      style={{ "--mark-size": `${size}px` } as React.CSSProperties}
    >
      <span className="syndra-mark-base" />
      <span className="syndra-mark-arc" />
      <span className={`syndra-mark-dot${bloom ? " bloom" : ""}`} />
    </span>
  );
}

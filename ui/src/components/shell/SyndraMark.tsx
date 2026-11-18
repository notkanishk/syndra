"use client";

import { useEffect, useRef, useState } from "react";

import type { Indicators } from "@/lib/queries/useIndicators";

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

/**
 * What the room is like, said by the dot in the rail at rest (A-24 Q5): violet when something
 * needs you (as built), lime when nothing does, amber and breathing while Zitadel isn't answering,
 * dim in quiet hours with nothing waiting. A dot, not words: the rail's own counts say what.
 */
export type Room = "needs" | "clear" | "waiting" | "quiet";

/**
 * Whether anything in the rail's reading waits on someone: every count it
 * carries, whichever view is showing (a queue Basic folds still waits), the
 * sends Syndra is retrying or gave up on, and the operations not confirmed.
 * Lime is said only when every one is zero. Two of Home's blocks are not in
 * this reading (a person who joined with nothing, a service account with no
 * owner): see A-24 tasks 7w-3, NEXT 0l.
 */
export function needsYou(indicators: Indicators): boolean {
  const counts = [
    indicators.pending_requests,
    indicators.expiring_grants,
    indicators.pending_propagation,
    indicators.drift,
    indicators.unconfirmed_revocations,
    indicators.holds_due,
    indicators.retrying,
    indicators.gave_up,
    ...Object.values(indicators.unconfirmed_operations ?? {}),
  ];
  return counts.some((count) => (count ?? 0) > 0);
}

/**
 * The room from what the rail reads. Zitadel not answering says itself first,
 * because then the counts may be wrong; with no real reading of the counts the
 * state is unknown (undefined), never "clear"; quiet hours dim only a room
 * where nothing waits, as Home's "It will keep until morning" does.
 *
 * "Nothing waits" (lime, and dim) is said only from a reading that covers
 * everything Home counts (`complete`). The rail's reading does not yet carry
 * all of Home's blocks (A-24 tasks 7w-3), so until it does the mark says
 * waiting and needs-you only, and is otherwise as built: never something untrue.
 */
export function roomOf(facts: { needs: boolean | undefined; degraded: boolean; quiet: boolean; complete: boolean }): Room | undefined {
  if (facts.degraded) return "waiting";
  if (facts.needs === undefined) return undefined;
  if (facts.needs) return "needs";
  if (!facts.complete) return undefined;
  return facts.quiet ? "quiet" : "clear";
}

export function SyndraMark({
  size = 22,
  className = "",
  welcome = false,
  room,
}: {
  size?: number;
  className?: string;
  welcome?: boolean;
  /** Unknown (undefined) draws the mark as built. */
  room?: Room;
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
      data-room={room}
      className={`syndra-mark ${className}`}
      style={{ "--mark-size": `${size}px` } as React.CSSProperties}
    >
      <span className="syndra-mark-base" />
      <span className="syndra-mark-arc" />
      {/* Breathe is the one loop, and it means still happening: Zitadel not answering is. */}
      <span className={`syndra-mark-dot${bloom ? " bloom" : ""}${room === "waiting" ? " breathe" : ""}`} />
    </span>
  );
}

"use client";

/**
 * You walk through the door (A-24 §3.2, Q4): the first page of a sign-in opens
 * on the sign-in door's own room, the same arch and the same violet light, and
 * the door opens around you. The arch widens past you and is gone; its light
 * floods the window and dissolves into the page's own light (Home's hour pool).
 *
 * Drawn and timed entirely in globals.css (`door-through*`, one keyframe set on
 * `--door-through`), in the door's always-dark look in both rooms. Nothing in
 * it travels: only its light's opacity and size change. It is decoration, so
 * it is hidden from assistive technology and never takes a click; the page is
 * under it from the first frame and tints in at `--door-through-reveal`. When
 * its light has dissolved it tells the shell, which takes it away.
 */
export function DoorThrough({ onDone }: { onDone: () => void }) {
  return (
    <div aria-hidden className="door-through">
      <div className="door-through-room" />
      {/* The light is the last to finish: its end is the door's. */}
      <div className="door-through-light" onAnimationEnd={onDone} />
      <div className="door-through-arch" />
    </div>
  );
}

"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import type { DayPhase } from "@/lib/greeting";
import { openedAlready } from "@/lib/motion/door";
import { DOOR_OPEN_MS } from "@/lib/motion/still";

/** The shell's column (AppShell), the one place the room's light is drawn. */
export const RoomContext = createContext<HTMLElement | null>(null);

/**
 * The room's light is the column's, not the page's (A-25 §3; R3, owner, 20 Nov 2026): a page hands its light
 * up to the column, behind the top bar and the scroller, so neither the bar's hairline, the
 * scroller's box nor its scrollbar's seat can cut it, and it stays while the page scrolls. Outside
 * the shell there is no column, and the light is not drawn: never inside the scroller.
 */
export function RoomLight({ children }: { children: ReactNode }) {
  const room = useContext(RoomContext);
  return room && createPortal(children, room);
}

/**
 * The hour's light, one path for Home and My access (A-24 §3.1 and §3.2, A-25 moment 1): the hour's
 * pool, and the door's light crossing it once after a sign-in. Mounted with the page's first render,
 * so it decides the door before the shell spends it; drawn once there is an hour (`phase`, on the
 * greeting's clock: null before it has read the clock) and the page says it may be lit (`lit`: My
 * access waits for a good read of Zitadel).
 *
 * How it arrives is decided once, the first time it is drawn: lit from the first render, it is there
 * at once (or crosses, if the door is due); lit later, it settles in, and crosses only if the door is
 * due and the wait was shorter than the crossing itself (`--door-open`). Drawn again after going, it
 * settles in: the door is the sign-in's first moment, never a later one.
 */
export function HourLight({
  phase,
  door,
  signedInAt,
  lit = true,
}: {
  phase: DayPhase | null;
  door: boolean;
  signedInAt?: string;
  lit?: boolean;
}) {
  // The server decides the door from the sign-in's cookie; a cached render (Back) of a door this
  // device has already opened stays still.
  const [due] = useState(() => door && !openedAlready(signedInAt));
  const [first] = useState(() => ({ lit, at: performance.now() }));
  const [arrival, setArrival] = useState<"cross" | "settle" | "still" | "gone" | null>(null);
  const shown = Boolean(phase) && lit;
  // Derived once from what the page says (React's state-from-props pattern): no effect, no frame without it.
  if (shown && (arrival === null || arrival === "gone")) {
    const prompt = first.lit || performance.now() - first.at <= DOOR_OPEN_MS;
    setArrival(arrival === null && due && prompt ? "cross" : arrival === null && first.lit ? "still" : "settle");
  } else if (!shown && arrival !== null && arrival !== "gone") {
    setArrival("gone");
  }
  if (!shown || arrival === null || arrival === "gone") return null;
  return (
    <RoomLight>
      <div
        aria-hidden
        data-hour={phase}
        data-door={arrival === "cross" ? "" : undefined}
        data-settle={arrival === "settle" ? "" : undefined}
        className="hour-light"
      />
    </RoomLight>
  );
}

"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import type { DayPhase } from "@/lib/greeting";
import { openedAlready } from "@/lib/motion/door";

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
 * greeting's clock: null before it has read the clock).
 */
export function HourLight({ phase, door, signedInAt }: { phase: DayPhase | null; door: boolean; signedInAt?: string }) {
  // The server decides the door from the sign-in's cookie; a cached render (Back) of a door this
  // device has already opened stays still.
  const [crossing] = useState(() => door && !openedAlready(signedInAt));
  return (
    phase && (
      <RoomLight>
        <div aria-hidden data-hour={phase} data-door={crossing ? "" : undefined} className="hour-light" />
      </RoomLight>
    )
  );
}

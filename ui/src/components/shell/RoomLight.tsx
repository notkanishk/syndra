"use client";

import { createContext, useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";

/** The shell's column (AppShell), the one place the room's light is drawn. */
export const RoomContext = createContext<HTMLElement | null>(null);

/**
 * The room's light is the column's, not the page's (A-25 §3, the owner's R3): a page hands its light
 * up to the column, behind the top bar and the scroller, so neither the bar's hairline, the
 * scroller's box nor its scrollbar's seat can cut it, and it stays while the page scrolls. Outside
 * the shell there is no column, and the light is not drawn: never inside the scroller.
 */
export function RoomLight({ children }: { children: ReactNode }) {
  const room = useContext(RoomContext);
  return room && createPortal(children, room);
}

"use client";

import { useCallback, useEffect, useRef } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { COPIED_MS } from "@/lib/motion";

/**
 * The words of every copy control (Motion v2 S18). One seat, sized to the wider
 * label so the button never changes width: the idle word crossfades with the
 * done word on tint. `dot` is for a copy that went somewhere, and it blooms
 * once; a selection is still the reader's to finish, so it says its word and
 * nothing more.
 */
export function CopiedSeat({
  done,
  idle,
  word,
  dot = true,
}: {
  done: boolean;
  idle: string;
  word: string;
  dot?: boolean;
}) {
  return (
    <span className="grid items-center justify-items-start [&>*]:col-start-1 [&>*]:row-start-1">
      <span aria-hidden={done} className={`motion-tint ${done ? "opacity-0" : "opacity-100"}`}>
        {idle}
      </span>
      <span
        aria-hidden={!done}
        className={`inline-flex items-center gap-2 motion-tint ${done ? "opacity-100" : "opacity-0"}`}
      >
        {dot && <StatusDot tone="healthy" bloom={done} />}
        {word}
      </span>
    </span>
  );
}

/**
 * Turns a flag on and puts it back after `COPIED_MS`, restarting if it is
 * pressed again, and clearing on unmount so a polled page that remounts never
 * sets state on a gone component.
 */
export function useConfirmFor(): (set: (on: boolean) => void) => void {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  return useCallback((set) => {
    set(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => set(false), COPIED_MS);
  }, []);
}

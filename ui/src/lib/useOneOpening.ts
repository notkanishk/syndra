"use client";

import { useRef } from "react";

/**
 * A clearing earns one opening of the all-clear's light and lift (A-25 moment 7), and no clock decides it.
 *
 * The first mount of the all-clear after the clearing spends it: the empty state tells the hook through
 * `spend`, so a slow leave (the last row held for the pointer, an outcome) still opens when the line finally
 * mounts. A read that stops answering after the clearing (`ready` false) spends it too: the line that comes
 * back with the next good read is the same clearing seen again. A queue that fills again (`earned` false)
 * re-arms it. Read `open` only to hand it to a line that has not mounted; `EmptyState` and `AllClear` keep
 * what they were first given, so a re-render never replays or cuts it short.
 */
export function useOneOpening(earned: boolean, ready: boolean) {
  const spent = useRef(false);
  if (!earned) spent.current = false;
  else if (!ready) spent.current = true;
  // `spend` is a new function each render; nothing depends on its identity.
  return { open: earned && !spent.current, spend: () => void (spent.current = true) };
}

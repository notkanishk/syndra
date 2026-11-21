"use client";

import { useRef, useState } from "react";

import { ownActAfter, ownActWithin } from "@/lib/query-client";
import { useOneOpening } from "@/lib/useOneOpening";

/** A send started this long ago, whose queue then empties, is this operator's. */
const MINE_WITHIN_MS = 30_000;

export interface QueueCleared<T> {
  /** Which clearing this is (1, 2, ...): a line that outlives one clearing is keyed by it. */
  id: number;
  /** What was in the queue when it emptied: it is what fades. */
  items: T[];
  /** This operator's send emptied it: how many they sent, and when it had gone. */
  mine: { count: number; at: string } | null;
}

/**
 * Notices a queue that empties while it is on screen (Motion v2 S13).
 *
 * A queue that was empty on arrival has nothing to say about it; only the
 * moment it clears is news. Whose doing it was is asked of the person's own
 * click, not guessed: `beginSend` marks the start of this operator's send and
 * `failedSend` takes it back, so a queue that empties under someone else's
 * send, or a cancel, is never claimed as ours. Anything that empties it is
 * `cleared`; only `mine` earns the count, and `glow` (the ember's one opening) while it is still the moment.
 *
 * A queue with no single "send" (the operator decides row by row) passes
 * `anyOwnAct`: then any write of the operator's own, landed in the last 30s,
 * counts as theirs. It is still never claimed on arrival or on a refresh.
 *
 * Clears itself when the queue fills again.
 *
 * `scope` is what the queue is of (Home's view, Basic or Advanced): when it
 * changes, the queue is a different one, so what it holds now is simply taken
 * as seen. A switch to a view with less in it never reads as having emptied it.
 */
export function useQueueCleared<T>(items: T[], ready: boolean, anyOwnAct = false, scope?: unknown) {
  const [seen, setSeen] = useState<T[]>([]);
  const [cleared, setCleared] = useState<QueueCleared<T> | null>(null);
  const [scoped, setScoped] = useState(scope);
  const sending = useRef<{ count: number; at: number } | null>(null);
  // When the queue last filled (or arrived): a write of theirs from before that is not what emptied it now.
  const armed = useRef(0);
  const clearings = useRef(0);

  if (scope !== scoped) {
    setScoped(scope);
    setSeen(items);
    if (cleared) setCleared(null);
    armed.current = Date.now();
    sending.current = null;
  } else if (ready && items.length > 0 && !sameItems(items, seen)) {
    if (cleared || seen.length === 0) {
      armed.current = Date.now();
      sending.current = null;
    }
    setSeen(items);
    if (cleared) setCleared(null);
  } else if (ready && items.length === 0 && seen.length > 0 && !cleared) {
    const started = sending.current;
    const mine =
      started && Date.now() - started.at < MINE_WITHIN_MS
        ? { count: started.count, at: new Date().toISOString() }
        : anyOwnAct && ownActWithin(MINE_WITHIN_MS) && ownActAfter(armed.current)
          ? { count: seen.length, at: new Date().toISOString() }
          : null;
    sending.current = null;
    setSeen([]);
    setCleared({ id: ++clearings.current, items: seen, mine });
  }

  // `mine` says whose it was and keeps the count; `glow` is the one opening it earns (`useOneOpening`).
  const { open: glow, spend } = useOneOpening(cleared?.mine != null, ready);

  return {
    cleared,
    glow,
    spend,
    beginSend: (count: number) => {
      sending.current = { count, at: Date.now() };
    },
    failedSend: () => {
      sending.current = null;
    },
  };
}

/** The same rows, even when the array holding them was rebuilt: a caller's memo may not be stable. */
function sameItems<T>(a: T[], b: T[]): boolean {
  return a === b || (a.length === b.length && a.every((item, i) => item === b[i]));
}

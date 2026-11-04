"use client";

import { useRef, useState } from "react";

import { ownActWithin } from "@/lib/query-client";

/** A send started this long ago, whose queue then empties, is this operator's. */
const MINE_WITHIN_MS = 30_000;

export interface QueueCleared<T> {
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
 * `cleared`; only `mine` earns the bloom and a count.
 *
 * A queue with no single "send" (the operator decides row by row) passes
 * `anyOwnAct`: then any write of the operator's own, landed in the last 30s,
 * counts as theirs. It is still never claimed on arrival or on a refresh.
 *
 * Clears itself when the queue fills again.
 */
export function useQueueCleared<T>(items: T[], ready: boolean, anyOwnAct = false) {
  const [seen, setSeen] = useState<T[]>([]);
  const [cleared, setCleared] = useState<QueueCleared<T> | null>(null);
  const sending = useRef<{ count: number; at: number } | null>(null);

  if (ready && items.length > 0 && !sameItems(items, seen)) {
    setSeen(items);
    if (cleared) setCleared(null);
  } else if (ready && items.length === 0 && seen.length > 0 && !cleared) {
    const started = sending.current;
    const mine =
      started && Date.now() - started.at < MINE_WITHIN_MS
        ? { count: started.count, at: new Date().toISOString() }
        : anyOwnAct && ownActWithin(MINE_WITHIN_MS)
          ? { count: seen.length, at: new Date().toISOString() }
          : null;
    sending.current = null;
    setSeen([]);
    setCleared({ items: seen, mine });
  }

  return {
    cleared,
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

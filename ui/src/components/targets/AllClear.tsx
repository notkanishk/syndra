"use client";

import { useState } from "react";

import { EmptyState } from "@/components/states";
import { UnfoldOnArrival } from "@/lib/motion";

/**
 * True once a list that had something in it has been drawn empty: it emptied in
 * front of the reader (D-1 M3), which is what lets its all-clear open, rise and
 * bloom. A list that was empty when it was first read is simply empty.
 * `undefined` is "not read yet".
 */
export function useEmptied(drawn: number | undefined): boolean {
  const [seen, setSeen] = useState(drawn);
  const [emptied, setEmptied] = useState(false);
  if (drawn !== seen) {
    setSeen(drawn);
    setEmptied(seen !== undefined && seen > 0 && drawn === 0);
  }
  return emptied;
}

/**
 * The all-clear of a queue: resolved, rising into its place. When the list
 * emptied in front of the reader its space opens first; and the dot blooms only
 * when the reader's own act emptied it.
 */
export function AllClear({
  title,
  guidance,
  emptied,
  byReader,
  inCard = false,
}: {
  title: string;
  guidance: string;
  emptied: boolean;
  byReader: boolean;
  /** The caller opens a card around it (AllClearCard), so the line need not open its own space. */
  inCard?: boolean;
}) {
  const line = <EmptyState resolved lift={emptied} bloom={emptied && byReader} title={title} guidance={guidance} />;
  // Its space is opened by the card around it when it sits in one (AllClearCard); alone, here.
  return emptied && !inCard ? <UnfoldOnArrival>{line}</UnfoldOnArrival> : line;
}

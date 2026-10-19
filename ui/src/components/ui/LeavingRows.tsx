"use client";

import { useState, type FocusEvent, type ReactNode } from "react";

import { stillMotion } from "@/lib/motion";

/**
 * A row that leaves on a later read (Motion v2 S5): it fades where it was, then
 * its gap closes on settle. Rows that stay are the same elements throughout.
 *
 * If the pointer or focus is inside the list, the gap waits until it leaves: a
 * list that closes up under a cursor is a list whose next click lands on the
 * wrong row.
 *
 * A list opts in: it passes its rows through `useLeavingRows`, renders what
 * comes back, and wraps each row in `LeavingRow`. Not every row is a direct
 * child of the list wrapper (Pending changes nests them in groups), so this
 * cannot be found from the outside.
 */
interface Kept<T> {
  row: T;
  key: string;
  leaving: boolean;
}

function merge<T>(before: Kept<T>[], rows: T[], keyOf: (row: T) => string): Kept<T>[] {
  const next = new Set(rows.map(keyOf));
  const out: Kept<T>[] = rows.map((row) => ({ row, key: keyOf(row), leaving: false }));
  before.forEach((was, index) => {
    if (next.has(was.key)) return;
    // Where it was: straight after the nearest row before it that is still shown.
    let at = 0;
    for (let back = index - 1; back >= 0; back--) {
      const found = out.findIndex((kept) => kept.key === before[back].key);
      if (found >= 0) {
        at = found + 1;
        break;
      }
    }
    out.splice(at, 0, { ...was, leaving: true });
  });
  return out;
}

export interface LeaveProps {
  leaving: boolean;
  held: boolean;
  onGone: () => void;
}

/**
 * `rows` is what to draw: the read's rows, with the ones that just left still
 * in their place. `props(key)` is what each row's `LeavingRow` takes;
 * `listProps` goes on the element that holds the rows, to know about the pointer
 * and focus. `ready` is false until the first read has landed, so a list that
 * has not loaded is not mistaken for one that emptied.
 */
export function useLeavingRows<T>(rows: T[], keyOf: (row: T) => string, ready = true) {
  const [kept, setKept] = useState<Kept<T>[]>(() => rows.map((row) => ({ row, key: keyOf(row), leaving: false })));
  const [seen, setSeen] = useState(rows);
  const [pointer, setPointer] = useState(false);
  const [focus, setFocus] = useState(false);

  if (rows !== seen) {
    setSeen(rows);
    setKept(
      ready && !stillMotion()
        ? merge(kept, rows, keyOf)
        : rows.map((row) => ({ row, key: keyOf(row), leaving: false })),
    );
  }

  const held = pointer || focus;
  const leftFocus = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocus(false);
  };

  return {
    rows: kept.map((entry) => entry.row),
    props: (key: string): LeaveProps => ({
      leaving: kept.some((entry) => entry.key === key && entry.leaving),
      held,
      onGone: () => setKept((now) => now.filter((entry) => !(entry.key === key && entry.leaving))),
    }),
    listProps: {
      onPointerEnter: () => setPointer(true),
      onPointerLeave: () => setPointer(false),
      onFocus: () => setFocus(true),
      onBlur: leftFocus,
    },
  };
}

export function LeavingRow({ leaving, held, onGone, children }: LeaveProps & { children: ReactNode }) {
  // Once the gap has started to close it carries on, whatever the pointer does.
  const [closing, setClosing] = useState(false);
  if (leaving && !held && !closing) setClosing(true);
  if (!leaving && closing) setClosing(false);

  return (
    <div
      className={closing ? "close-gap" : undefined}
      aria-hidden={leaving ? true : undefined}
      inert={leaving ? true : undefined}
      onAnimationEnd={(event) => {
        if (closing && event.target === event.currentTarget) onGone();
      }}
    >
      <div className={leaving ? "tint-out" : undefined}>{children}</div>
    </div>
  );
}

"use client";

import { useState, type FocusEvent, type PointerEvent, type ReactNode } from "react";

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
  /** No longer in the read. It leaves once nothing holds it (see `stay`). */
  gone: boolean;
}

const NOBODY: ReadonlySet<string> = new Set();

function merge<T>(
  before: Kept<T>[],
  rows: T[],
  keyOf: (row: T) => string,
  stay: ReadonlySet<string>,
  still: boolean,
): Kept<T>[] {
  const next = new Set(rows.map(keyOf));
  const out: Kept<T>[] = rows.map((row) => ({ row, key: keyOf(row), gone: false }));
  before.forEach((was, index) => {
    if (next.has(was.key)) return;
    // Nothing to fade, so nothing to keep: only a row held for its outcome stays.
    if (still && !stay.has(was.key)) return;
    // Where it was: straight after the nearest row before it that is still shown.
    let at = 0;
    for (let back = index - 1; back >= 0; back--) {
      const found = out.findIndex((kept) => kept.key === before[back].key);
      if (found >= 0) {
        at = found + 1;
        break;
      }
    }
    out.splice(at, 0, { ...was, gone: true });
  });
  return out;
}

export interface LeaveProps {
  leaving: boolean;
  held: boolean;
  onGone: () => void;
}

export interface LeavingOptions {
  /**
   * Keys whose row has dropped out of the read but is still showing something
   * the reader has to see first (a decision's outcome, Motion 5b's beat). A held
   * row stays as it is, buttons and all, and leaves like any other once it is
   * released. Under reduced motion it leaves at once, with no fade.
   */
  stay?: ReadonlySet<string>;
  /**
   * What the rows are a reading of (a filter's value). When it changes the list
   * is a different list: nothing fades out of it, the old rows are just gone.
   */
  scope?: string;
}

/**
 * `rows` is what to draw: the read's rows, with the ones that just left still
 * in their place. `props(key)` is what each row's `LeavingRow` takes;
 * `listProps` goes on the element that holds the rows, to know about the pointer
 * and focus. `ready` is false until the first read has landed, so a list that
 * has not loaded is not mistaken for one that emptied.
 */
export function useLeavingRows<T>(
  rows: T[],
  keyOf: (row: T) => string,
  ready = true,
  { stay = NOBODY, scope }: LeavingOptions = {},
) {
  const [kept, setKept] = useState<Kept<T>[]>(() => rows.map((row) => ({ row, key: keyOf(row), gone: false })));
  const [seen, setSeen] = useState(rows);
  const [was, setWas] = useState(scope);
  const [pointer, setPointer] = useState(false);
  const [focus, setFocus] = useState(false);

  const other = scope !== was;
  // A new array of the same rows (a memo that re-ran, a sort that changed nothing) is not a read.
  const same = rows.length === seen.length && rows.every((row, index) => row === seen[index]);
  if ((rows !== seen && !same) || other) {
    setSeen(rows);
    setWas(scope);
    setKept(
      ready && !other
        ? merge(kept, rows, keyOf, stay, stillMotion())
        : rows.map((row) => ({ row, key: keyOf(row), gone: false })),
    );
  }

  const leaves = (entry: Kept<T>) => entry.gone && !stay.has(entry.key);
  // Nothing animates under reduced motion, so a row that is going is already gone.
  const shown = stillMotion() ? kept.filter((entry) => !leaves(entry)) : kept;

  const held = pointer || focus;
  const leftFocus = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocus(false);
  };

  return {
    rows: shown.map((entry) => entry.row),
    props: (key: string): LeaveProps => ({
      leaving: kept.some((entry) => entry.key === key && leaves(entry)),
      held,
      onGone: () => setKept((now) => now.filter((entry) => !(entry.key === key && entry.gone))),
    }),
    listProps: {
      onPointerEnter: () => setPointer(true),
      // A focused button that is removed (Approve, once decided) never says it
      // blurred, so focus is read again here rather than trusted: otherwise the
      // gap would wait on a focus that is no longer anywhere.
      onPointerLeave: (event: PointerEvent<HTMLElement>) => {
        setPointer(false);
        setFocus(event.currentTarget.contains(document.activeElement));
      },
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

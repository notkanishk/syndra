"use client";

import { useEffect, useRef, useState, type FocusEvent, type PointerEvent, type ReactNode } from "react";

import { ARRIVE_MEMORY_MS, armedRecently } from "@/components/ui/ArriveList";
import { SETTLE_MS, TINT_MS, stillMotion } from "@/lib/motion";

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
  /** Came in on a later read, not with the list: it tints in where it lands. */
  fresh: boolean;
  /** Dropped out because the reader changed what they are looking through: its gap never waits on the pointer. */
  eager: boolean;
}

const NOBODY: ReadonlySet<string> = new Set();

function merge<T>(
  before: Kept<T>[],
  rows: T[],
  keyOf: (row: T) => string,
  stay: ReadonlySet<string>,
  drawn: ReadonlySet<string>,
  still: boolean,
  arrives: boolean,
  byReader: boolean,
): Kept<T>[] {
  const next = new Set(rows.map(keyOf));
  const known = new Map(before.map((was) => [was.key, was]));
  // Rows that stay keep what they were; only a key not seen before is new. One that is back
  // before its fade ended is new too: it was leaving, and it tints in rather than snapping back.
  const out: Kept<T>[] = rows.map((row) => {
    const key = keyOf(row);
    const was = known.get(key);
    return { row, key, gone: false, fresh: was ? was.fresh || (was.gone && !stay.has(key)) : arrives, eager: false };
  });
  before.forEach((was, index) => {
    if (next.has(was.key)) return;
    // Nothing to fade, so nothing to keep: only a row held for its outcome stays.
    if (still && !stay.has(was.key)) return;
    // A row that was never on screen (past the page's limit, in a closed fold) has nothing
    // to fade either. Kept, it would take a shown row's seat and then fade in from nowhere.
    if (!drawn.has(was.key) && !stay.has(was.key)) return;
    // Where it was: straight after the nearest row before it that is still shown.
    let at = 0;
    for (let back = index - 1; back >= 0; back--) {
      const found = out.findIndex((kept) => kept.key === before[back].key);
      if (found >= 0) {
        at = found + 1;
        break;
      }
    }
    out.splice(at, 0, { ...was, gone: true, eager: was.gone ? was.eager : byReader });
  });
  return out;
}

export interface LeaveProps {
  leaving: boolean;
  /** New on a later read: it tints in. Rows the reader asked for arrive instead (see `armArrive`). */
  fresh?: boolean;
  held: boolean;
  onGone: () => void;
  /** Tells the list this row is on screen: only a row that was drawn can fade out. */
  track?: (drawn: boolean) => void;
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
   * What the rows are a reading of, for a list whose tabs are different reads
   * (Requests' status). When it changes the list is a different list: nothing
   * fades out of it, the old rows are just gone.
   */
  scope?: string;
  /**
   * What the reader is looking through (a filter's value, a search). Not a
   * different list: the rows that drop out leave like any other, and the rows it
   * brings in tint in. But their gap does not wait for the pointer: it is on the
   * reader's own control, or parked over the list they are narrowing, and a gap
   * held open there is a hole. The window is `ARRIVE_MEMORY_MS`, because a
   * search's read lands after the keystroke (A-05 §2.10).
   */
  narrowedBy?: string;
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
  { stay = NOBODY, scope, narrowedBy }: LeavingOptions = {},
) {
  const [kept, setKept] = useState<Kept<T>[]>(() => rows.map((row) => ({ row, key: keyOf(row), gone: false, fresh: false, eager: false })));
  // Rows are new only once the list has been read: what a first read brings is
  // the list arriving, not rows that came in.
  const [read, setRead] = useState(ready);
  const [seen, setSeen] = useState(rows);
  const [was, setWas] = useState(scope);
  const [narrowedWas, setNarrowedWas] = useState(narrowedBy);
  const [narrowedAt, setNarrowedAt] = useState(0);
  const [pointer, setPointer] = useState(false);
  const [focus, setFocus] = useState(false);
  // Which rows are on screen now, from each LeavingRow's own mount. A list draws a window
  // of its rows (a page's limit, open folds), and only the hook's caller knows which.
  const drawn = useRef(new Set<string>());

  const other = scope !== was;
  const narrowed = narrowedBy !== narrowedWas;
  const byReader = narrowed || Date.now() - narrowedAt <= ARRIVE_MEMORY_MS;
  // A new array of the same rows (a memo that re-ran, a sort that changed nothing) is not a read.
  const same = rows.length === seen.length && rows.every((row, index) => row === seen[index]);
  if ((rows !== seen && !same) || other || narrowed) {
    setSeen(rows);
    setWas(scope);
    if (narrowed) {
      setNarrowedWas(narrowedBy);
      setNarrowedAt(Date.now());
    }
    setKept(
      ready && !other
        ? merge(kept, rows, keyOf, stay, drawn.current, stillMotion(), read && !armedRecently(), byReader)
        : rows.map((row) => ({ row, key: keyOf(row), gone: false, fresh: false, eager: false })),
    );
  }
  if (ready && !read) setRead(true);

  const leaves = (entry: Kept<T>) => entry.gone && !stay.has(entry.key);
  // Nothing animates under reduced motion, so a row that is going is already gone.
  const shown = stillMotion() ? kept.filter((entry) => !leaves(entry)) : kept;

  // A leaving row pushed out of the window (rows came in above it) will never end its own
  // fade: it goes now. Rows report their mount before this runs, children first.
  useEffect(() => {
    if (kept.some((entry) => entry.gone && !stay.has(entry.key) && !drawn.current.has(entry.key))) {
      setKept((now) => now.filter((entry) => !(entry.gone && !stay.has(entry.key) && !drawn.current.has(entry.key))));
    }
  }, [kept, stay]);

  const held = pointer || focus;
  const leftFocus = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocus(false);
  };

  return {
    rows: shown.map((entry) => entry.row),
    /**
     * The first `limit` rows of the read, with the leaving rows among them, for a list that
     * draws a page of its rows. A row on its way out never takes a row's seat on the page.
     */
    window: (limit: number) => {
      const out: T[] = [];
      let live = 0;
      for (const entry of shown) {
        if (leaves(entry)) {
          if (live < limit) out.push(entry.row);
          continue;
        }
        if (live === limit) break;
        out.push(entry.row);
        live++;
      }
      return out;
    },
    props: (key: string): LeaveProps => {
      const entry = kept.find((one) => one.key === key);
      return {
        leaving: entry ? leaves(entry) : false,
        fresh: entry?.fresh ?? false,
        held: held && !entry?.eager,
        onGone: () => setKept((now) => now.filter((one) => !(one.key === key && one.gone))),
        track: (on) => {
          if (on) drawn.current.add(key);
          else drawn.current.delete(key);
        },
      };
    },
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

export function LeavingRow({ leaving, fresh, held, onGone, track, children }: LeaveProps & { children: ReactNode }) {
  // Once the gap has started to close it carries on, whatever the pointer does.
  const [closing, setClosing] = useState(false);
  if (leaving && !held && !closing) setClosing(true);
  if (!leaving && closing) setClosing(false);

  // The gap is gone when its fold ends. If no animation runs to say so (none
  // matched, or the styles never loaded), it is removed after the fold's own
  // delay and duration anyway rather than left as an inert row.
  // `onGone` is new on every render, so it is read through a ref: a parent
  // that re-renders often must not keep restarting the wait.
  const gone = useRef(onGone);
  useEffect(() => {
    gone.current = onGone;
  });
  const tracked = useRef(track);
  useEffect(() => {
    tracked.current?.(true);
    return () => tracked.current?.(false);
  }, []);
  useEffect(() => {
    if (!closing) return;
    const timer = setTimeout(() => gone.current(), TINT_MS + SETTLE_MS + 100);
    return () => clearTimeout(timer);
  }, [closing]);

  return (
    <div
      className={closing ? "close-gap" : undefined}
      aria-hidden={leaving ? true : undefined}
      inert={leaving ? true : undefined}
      onAnimationEnd={(event) => {
        if (closing && event.target === event.currentTarget) onGone();
      }}
    >
      <div className={leaving ? "tint-out" : fresh ? "tint-in" : undefined}>{children}</div>
    </div>
  );
}

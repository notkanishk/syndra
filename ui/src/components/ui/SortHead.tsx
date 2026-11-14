"use client";

import { useEffect, useRef, useState } from "react";

import { stillMotion, TINT_MS } from "@/lib/motion";
import { keepView, recallView } from "@/lib/place";
import { compareNames } from "@/lib/sort";

/**
 * Sorting by a column heading (RULES: sort lives on the headings).
 *
 * `useSort` holds which heading is in force and which way; pressing the one in
 * force reverses it, pressing another starts it in its first order: ascending,
 * or descending for a heading listed in `descending` (a time newest first, an
 * amount largest first), so the arrow and `aria-sort` say what the rows do. `SortHead` is the
 * heading itself, carrying the arrow only while it is the one in force. And
 * where a sort gathers equal values, `runStarts` says which rows begin a run
 * and how big the run is, and `RunLabel` is the quiet counted label above it.
 *
 * What each key means, and how rows compare under it, stays with the caller:
 * this module never sees a row.
 */
export type Sort<K extends string> = { key: K; dir: 1 | -1 };

/**
 * Last view (A-19 P8): a list that names itself keeps its sort across visits,
 * as a per-browser preference. `keys` are the headings that exist, so a stale
 * value from an older build is ignored; `when: false` skips the memory where a
 * link has already chosen the order.
 */
export type RememberSort<K extends string> = { list: string; keys: readonly K[]; when?: boolean };

const isSortOf =
  <K extends string>(keys: readonly K[]) =>
  (value: unknown): value is Sort<K> => {
    const v = value as Sort<K> | null;
    return typeof v === "object" && v !== null && keys.includes(v.key) && (v.dir === 1 || v.dir === -1);
  };

/**
 * Where a list keeps its sort. A list with a heading that starts descending
 * keeps it apart ("sort.v2"): before those headings descended, the same lists
 * stored a newest-first time as ascending, and that value read now would open
 * oldest first. Read once and dropped, it costs one press.
 */
const sortSlot = (descending: readonly string[]) => (descending.length > 0 ? "sort.v2" : "sort");

export function useSort<K extends string>(initial: K, remember?: RememberSort<K>, descending: readonly K[] = []) {
  const first = (key: K): 1 | -1 => (descending.includes(key) ? -1 : 1);
  const slot = sortSlot(descending);
  const [sort, setSort] = useState<Sort<K>>(() => {
    const kept =
      remember && remember.when !== false ? recallView(remember.list, slot, isSortOf(remember.keys)) : undefined;
    return kept ?? { key: initial, dir: first(initial) };
  });
  // Only the reader's own press is a preference; the starting order is not.
  const pressed = useRef(false);
  const flip = (key: K) => {
    pressed.current = true;
    setSort((cur) => ({ key, dir: cur.key === key ? (-cur.dir as 1 | -1) : first(key) }));
  };
  const list = remember?.list;
  useEffect(() => {
    if (list && pressed.current) keepView(list, slot, sort);
  }, [list, slot, sort]);

  // A re-sort (A-05 §2.10): the heading turns at once, but the list fades out,
  // re-orders while nobody can see it, and fades back in. Rows never travel.
  // `view.sort` is what the rows are ordered by; `view.className` goes on the
  // element that holds them (never on the headings), or `view.rowClassName` on
  // each row when there is no such element.
  const [shown, setShown] = useState(sort);
  const [phase, setPhase] = useState<"out" | "in" | null>(null);
  useEffect(() => {
    if (sort === shown || stillMotion()) return;
    setPhase("out");
    const timer = setTimeout(() => {
      setShown(sort);
      setPhase("in");
    }, TINT_MS);
    return () => clearTimeout(timer);
  }, [sort, shown]);
  useEffect(() => {
    if (phase !== "in") return;
    const timer = setTimeout(() => setPhase(null), TINT_MS);
    return () => clearTimeout(timer);
  }, [phase]);

  const fade = phase === "out" ? "tint-out" : phase === "in" ? "tint-in" : "";
  const view = {
    sort: stillMotion() ? sort : shown,
    className: fade,
    // For rows that are direct children of an `ArriveList`, which stops their
    // animation once settled: the fade has to outrank it.
    rowClassName: phase === "out" ? "tint-out!" : phase === "in" ? "tint-in!" : "",
  };
  return [sort, flip, view] as const;
}

/**
 * The rows in the order a sort asks for, for a list that sorts as a whole.
 * Text compares as a person reads it (case and accents aside, "Lab 2" before
 * "Lab 10"); a row with no value (null) sits last whichever way; ties keep
 * the order the list already had, so a re-sort never shuffles equals.
 */
export function orderBy<T, K extends string>(
  rows: readonly T[],
  sort: Sort<K>,
  by: Record<K, (row: T) => string | number | null>,
): T[] {
  const get = by[sort.key];
  return [...rows].sort((a, b) => {
    const x = get(a);
    const y = get(b);
    if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
    const order =
      typeof x === "number" && typeof y === "number"
        ? x - y
        : compareNames(String(x), String(y));
    return order * sort.dir;
  });
}

/**
 * A column heading that sorts. `by` is what a screen reader hears after "Sort
 * by"; it is the label, lower-cased, unless a shorter name is wanted ("what"
 * for "What they can get into").
 */
export function SortHead<K extends string>({
  label,
  name,
  sort,
  onSort,
  by = label.toLowerCase(),
}: {
  label: string;
  name: K;
  sort: Sort<K>;
  onSort: (key: K) => void;
  by?: string;
}) {
  const on = sort.key === name;
  return (
    // aria-sort belongs to the header cell, not to the button inside it: the cell is what a
    // screen reader reads as "sorted ascending", the button is only how it is changed.
    <span role="columnheader" aria-sort={on ? (sort.dir > 0 ? "ascending" : "descending") : "none"}>
      <button
        type="button"
        aria-label={`Sort by ${by}`}
        data-sort={on ? (sort.dir > 0 ? "asc" : "desc") : "none"}
        onClick={() => onSort(name)}
        className={`inline-flex items-center gap-[5px] text-left uppercase tracking-[inherit] motion-tint hover:text-ink ${
          on ? "text-ink" : ""
        }`}
      >
        {label}
        <span
          aria-hidden
          className={`inline-block text-[11px] motion-turn ${on ? "opacity-100" : "opacity-0"} ${
            on && sort.dir < 0 ? "rotate-180" : ""
          }`}
        >
          ↑
        </span>
      </button>
    </span>
  );
}

/**
 * Which rows begin a run, in the order given. `runOf` is null when the sort
 * gathers nothing (it is by time, or by a heading whose values are all
 * different). The size is the run's whole size across `rows`, not how much of
 * it is on screen yet, so call it with every row and index it by position in a
 * paged prefix.
 */
export function runStarts<T>(rows: T[], runOf: ((row: T) => string) | null) {
  if (!runOf) return () => null;
  const sizes = new Map<string, number>();
  for (const row of rows) sizes.set(runOf(row), (sizes.get(runOf(row)) ?? 0) + 1);
  return (index: number) => {
    const run = runOf(rows[index]);
    return index === 0 || runOf(rows[index - 1]) !== run ? { label: run, size: sizes.get(run) ?? 0 } : null;
  };
}

/** The label above a run: its value and, quieter, how many are in it. */
export function RunLabel({ label, size }: { label: string; size: number }) {
  return (
    <div className="row-divider flex items-baseline gap-2.5 bg-surface-0 px-5 pb-2 pt-3">
      <span data-testid="run-label" className="type-label text-ink">
        {label}
        <span className="ml-2.5 text-[12.5px] font-semibold normal-case tracking-normal text-muted">
          {size}
        </span>
      </span>
    </div>
  );
}

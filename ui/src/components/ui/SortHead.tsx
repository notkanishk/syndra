"use client";

import { useState } from "react";

/**
 * Sorting by a column heading (RULES: sort lives on the headings).
 *
 * `useSort` holds which heading is in force and which way; pressing the one in
 * force reverses it, pressing another starts it ascending. `SortHead` is the
 * heading itself, carrying the arrow only while it is the one in force. And
 * where a sort gathers equal values, `runStarts` says which rows begin a run
 * and how big the run is, and `RunLabel` is the quiet counted label above it.
 *
 * What each key means, and how rows compare under it, stays with the caller:
 * this module never sees a row.
 */
export type Sort<K extends string> = { key: K; dir: 1 | -1 };

export function useSort<K extends string>(initial: K) {
  const [sort, setSort] = useState<Sort<K>>({ key: initial, dir: 1 });
  const flip = (key: K) =>
    setSort((cur) => ({ key, dir: cur.key === key ? (-cur.dir as 1 | -1) : 1 }));
  return [sort, flip] as const;
}

/**
 * The rows in the order a sort asks for, for a list that sorts as a whole.
 * Text compares as a person reads it (case and accents aside, "Lab 2" before
 * "Lab 10"); ties keep the order the list already had, so a re-sort never
 * shuffles equals.
 */
export function orderBy<T, K extends string>(
  rows: readonly T[],
  sort: Sort<K>,
  by: Record<K, (row: T) => string | number>,
): T[] {
  const get = by[sort.key];
  return [...rows].sort((a, b) => {
    const x = get(a);
    const y = get(b);
    const order =
      typeof x === "number" && typeof y === "number"
        ? x - y
        : String(x).localeCompare(String(y), undefined, { sensitivity: "base", numeric: true });
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
          className={`inline-block text-[11px] motion-press ${on ? "opacity-100" : "opacity-0"} ${
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

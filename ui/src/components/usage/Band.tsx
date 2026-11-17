"use client";

import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from "react";

import { dayBefore, dayGap, dayKept, dayNoValue, dayPending, dayValue, limitWord, sparkLabel } from "@/components/usage/copy";
import { stillMotion } from "@/lib/motion";
import type { StatsMetric, StatsUnit } from "@/lib/queries/useStats";
import { dayWords, formatValue, type Cell, type Plot } from "@/lib/usage";

/**
 * One series over the last 30 makerspace days, as a band 40px tall (A-23 §3.2;
 * dataviz: a count per day is columns, a level over time is a line). The width
 * is the seat's, measured, so every band in a card shares its left and right
 * edge and a day is one stripe down the card.
 *
 * Four kinds of day (§3.4). A counted day is a column (a real 0 a 1.5px stub)
 * or a point on the line. A day inside the record nobody could count is
 * hatched full height and the line BREAKS there, so a gap never reads as a
 * zero. A day after the last final one is blank, the baseline running on. A
 * day before the record has the window's baseline only, in `--line-strong`
 * (`--line` is 6-8% alpha and vanished on real data); the
 * record's strong baseline starts at a 6px tick. A level is never
 * interpolated: only adjacent counted days are joined, a value standing alone
 * is a dot. With a limit the scale's top is the limit, a faint line there and
 * its word beside it.
 *
 * The band is one focusable image. Hover, focus, ← and → read a day; the
 * words go to the section's one readout (`onRead`), and the read day's column
 * takes `--tint-3`. Colour only: nothing moves. A day that gained a value on
 * a later read tints in; on the first paint everything is drawn complete.
 */
const H = 40;
const PAD_Y = 6;
/** With a limit, its line sits this far down, its word above it, so the word never lies on the line's points. */
const LIMIT_Y = 16;
const BASE = H - PAD_Y;
/** The width before the seat is measured (and where it cannot be). */
const FALLBACK_W = 320;
/** A column's share of a day's step. */
const COLUMN = 0.45;

export function Band({
  plot,
  instrument,
  unit,
  what,
  keptDays,
  limit,
  readIndex,
  onRead,
}: {
  plot: Plot;
  instrument: StatsMetric["instrument"];
  unit: StatsUnit;
  /** What the band is, for the screen reader; the numbers are beside it. */
  what: string;
  /** How many days per-person numbers are kept, for a day Syndra no longer has. */
  keptDays?: number;
  /** A limit Syndra read, above 0: the scale's top. */
  limit?: number;
  /** The day being read, or null. */
  readIndex: number | null;
  onRead: (index: number | null) => void;
}) {
  const id = useId();
  const seat = useRef<HTMLDivElement>(null);
  const [W, setW] = useState(FALLBACK_W);
  useLayoutEffect(() => {
    const el = seat.current;
    if (!el) return;
    const measure = () => el.clientWidth > 0 && setW(el.clientWidth);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const watch = new ResizeObserver(measure);
    watch.observe(el);
    return () => watch.disconnect();
  }, []);

  // The days that had a value on the last paint: one that gains one later tints in (§7).
  const had = useRef<Set<string> | null>(null);
  useEffect(() => {
    had.current = new Set(plot.cells.filter((c) => c.value !== undefined).map((c) => c.day));
  });
  const fresh = (c: Cell) => (had.current !== null && !had.current.has(c.day) && !stillMotion() ? "tint-in" : undefined);

  const cells = plot.cells;
  const n = cells.length;
  const bars = instrument === "counter";
  // A column's half-width is the band's inset, so the first and last columns end at its edges.
  const padX = bars ? Math.max(3, (COLUMN * (W - 6)) / (n - 1) / 2) : 3;
  const step = (W - 2 * padX) / (n - 1);
  const x = (i: number) => padX + i * step;
  const ceiling = Math.max(limit ?? 0, 1, ...cells.map((c) => c.value ?? 0));
  const roof = limit !== undefined ? LIMIT_Y : PAD_Y;
  const y = (v: number) => BASE - (v / ceiling) * (BASE - roof);
  const left = (i: number) => Math.max(0, x(i) - step / 2);
  const right = (i: number) => Math.min(W, x(i) + step / 2);
  const first = cells.findIndex((c) => !c.before);
  const baseLeft = first < 0 ? W : first === 0 ? 0 : left(first);

  // Runs of adjacent cells with a value; any cell without one ends the run.
  const runs: Array<Array<[number, number, number]>> = [[]];
  cells.forEach((c, i) => {
    if (c.value === undefined) {
      if (runs[runs.length - 1].length) runs.push([]);
    } else runs[runs.length - 1].push([x(i), y(c.value), i]);
  });
  const lines = runs.filter((run) => run.length > 1);
  const last = cells.findLastIndex((c) => c.value !== undefined);
  const dots = new Set(runs.filter((run) => run.length === 1).map((run) => run[0][2]));
  if (last >= 0) dots.add(last);
  const bw = step * COLUMN;
  // The limit's word sits above its line, at the right; where a value over the limit (a folder past full)
  // would rise into it there, at the line's left; where it would at both ends, it is not drawn (the meter
  // above says "full" in words). ~7px a character at 12.5px.
  const word = limit !== undefined ? limitWord(formatValue(limit, unit)) : "";
  const span = word.length * 7 + 4;
  const over = (from: number, to: number) => cells.some((c, i) => c.value !== undefined && c.value > (limit ?? Infinity) && x(i) >= from && x(i) <= to);
  const wordAt: "end" | "start" | null = !over(W - span, W) ? "end" : !over(baseLeft, baseLeft + span) ? "start" : null;

  const keys = (e: KeyboardEvent) => {
    const by = e.key === "ArrowLeft" ? -1 : e.key === "ArrowRight" ? 1 : 0;
    if (!by) return;
    e.preventDefault();
    onRead(Math.min(n - 1, Math.max(0, (readIndex ?? n - 1) + by)));
  };

  return (
    <div ref={seat} className="relative h-10 min-w-0">
      {/* The day being read, behind the band: the board's inner radius (--radius-inner), colour only. It
          comes first and the band is positioned after it, so the band paints over it; it never takes the
          pointer, which stays the band's (a tint that took it would end the read it shows). */}
      {readIndex !== null && <div data-read aria-hidden className="pointer-events-none absolute rounded-inner bg-tint-3" style={{ insetBlockStart: 0, left: left(readIndex), width: right(readIndex) - left(readIndex), height: BASE }} />}
      <svg
        role="img"
        aria-label={sparkLabel(what, plot.gapRuns.length > 0)}
        tabIndex={0}
        width={W}
        height={H}
        viewBox={`0 0 ${W} ${H}`}
        className="relative block overflow-visible rounded-inner focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        onFocus={() => onRead(readIndex ?? n - 1)}
        onBlur={() => onRead(null)}
        onKeyDown={keys}
        onPointerLeave={() => onRead(null)}
      >
        <defs>
          <pattern id={`${id}-gap`} width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <line x1="0" y1="0" x2="0" y2="6" stroke="var(--chart-hatch)" strokeWidth="1" />
          </pattern>
        </defs>
        {plot.gapRuns.map(([a, b]) => (
          <rect key={a} data-gap x={left(a)} width={right(b) - left(a)} y="0" height={BASE} fill={`url(#${id}-gap)`} />
        ))}
        {baseLeft > 0 && <line data-pre x1="0" x2={baseLeft} y1={BASE + 0.5} y2={BASE + 0.5} stroke="var(--line-strong)" strokeWidth="1" />}
        {first >= 0 && <line data-base x1={baseLeft} x2={W} y1={BASE + 0.5} y2={BASE + 0.5} stroke="var(--line-strong)" strokeWidth="1" />}
        {first > 0 && <line data-tick x1={baseLeft + 0.5} x2={baseLeft + 0.5} y1={BASE - 5} y2={BASE + 1} stroke="var(--line-strong)" strokeWidth="1" />}
        {limit !== undefined && first >= 0 && (
          <>
            <line data-limit x1={baseLeft} x2={W} y1={y(limit) + 0.5} y2={y(limit) + 0.5} stroke="var(--line)" strokeWidth="1" />
            {wordAt && (
              <text data-limit-word x={wordAt === "start" ? baseLeft : W} y={y(limit) - 4} textAnchor={wordAt} fontSize="12.5" fill="var(--faint)">
                {word}
              </text>
            )}
          </>
        )}
        {bars
          ? cells.map((c, i) =>
              c.value === undefined ? null : (
                <rect key={c.day} data-column className={fresh(c)} x={x(i) - bw / 2} width={bw} y={BASE - Math.max(1.5, BASE - y(c.value))} height={Math.max(1.5, BASE - y(c.value))} rx="1" fill="var(--accent)" />
              ),
            )
          : lines.map((run) => (
              <g key={run[0][2]}>
                <polygon data-area points={`${run.map(([px, py]) => `${px},${py}`).join(" ")} ${run[run.length - 1][0]},${BASE} ${run[0][0]},${BASE}`} fill="var(--accent-soft)" />
                <polyline data-line points={run.map(([px, py]) => `${px},${py}`).join(" ")} fill="none" stroke="var(--accent)" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />
              </g>
            ))}
        {!bars &&
          [...dots].map((i) => (
            <circle key={i} data-dot className={fresh(cells[i])} cx={x(i)} cy={y(cells[i].value!)} r="2.5" fill="var(--accent)" stroke="var(--surface-1)" strokeWidth="1.5" />
          ))}
        {/* A hit target a whole day wide, so a day is readable without aiming at a 2px line. */}
        {cells.map((c, i) => (
          <rect key={c.day} x={left(i)} width={right(i) - left(i)} y="0" height={H} fill="transparent" onPointerEnter={() => onRead(i)}>
            <title>{cellWords(c, unit, keptDays)}</title>
          </rect>
        ))}
      </svg>
    </div>
  );
}

/** What a day says, in its title and in the section's readout (§3.4). */
export function cellWords(c: Cell, unit: StatsUnit, keptDays: number | undefined): string {
  const day = dayWords(c.day);
  if (c.before === "kept" && keptDays !== undefined) return dayKept(day, keptDays);
  if (c.before) return dayBefore(day);
  if (c.gap) return dayGap(day);
  if (c.value !== undefined) return dayValue(day, formatValue(c.value, unit));
  return c.pending ? dayPending(day) : dayNoValue(day);
}

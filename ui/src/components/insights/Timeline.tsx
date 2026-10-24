"use client";

import { addDays, dayLabel, daysBetween, isFirstOfMonth, monthLabel } from "@/components/insights/days";

/** Where a point a fraction along the track sits: the native thumb never reaches the ends. */
const at = (f: number) => `calc(8px + (100% - 16px) * ${f})`;

/**
 * The date under the map. One native range input does the work (keyboard,
 * touch, a slider's role); it is drawn over, not replaced, so what is seen is
 * the board's thin line, a tick for each day with changes, and a violet thumb.
 * Ticks after the chosen day are violet: those are the changes since.
 */
export function Timeline({
  from,
  to,
  value,
  changeDays,
  onChange,
}: {
  from: string;
  /** Today. */
  to: string;
  /** The date shown, or null for today. */
  value: string | null;
  /** Days that have changes. */
  changeDays: string[];
  onChange: (day: string | null) => void;
}) {
  const last = daysBetween(from, to);
  const shown = value ?? to;
  const here = Math.min(Math.max(daysBetween(from, shown), 0), last);
  const f = (day: string) => Math.min(Math.max(daysBetween(from, day), 0), last) / last;
  const months = Array.from({ length: last + 1 }, (_, i) => addDays(from, i)).filter(isFirstOfMonth);

  return (
    <div className="tint-in flex flex-col gap-3 rounded-card border border-line bg-surface-1 px-5 py-3.5 tablet:flex-row tablet:items-center tablet:gap-[18px]">
      <span className="type-label flex-none">Date</span>
      <div className="relative h-[42px] w-full flex-1">
        <input
          type="range"
          min={0}
          max={last}
          step={1}
          value={here}
          aria-label="Date shown"
          aria-valuetext={value ? dayLabel(value) : "Today"}
          onChange={(e) => {
            const n = Number(e.target.value);
            onChange(n >= last ? null : addDays(from, n));
          }}
          className="peer absolute inset-x-0 top-0 z-10 h-[42px] w-full cursor-grab opacity-0"
        />
        <span aria-hidden className="pointer-events-none absolute inset-x-0 top-3 h-0.5 rounded-sm bg-line-strong" />
        <span
          aria-hidden
          className="pointer-events-none absolute right-0 top-3 h-0.5 bg-accent-line"
          style={{ left: at(here / last) }}
        />
        {changeDays.map((day) => (
          <span
            key={day}
            aria-hidden
            data-tick={day}
            className={`pointer-events-none absolute top-[9px] h-2 w-0.5 -translate-x-px rounded-[1px] ${
              day > shown ? "bg-accent-text" : "bg-faint"
            }`}
            style={{ left: at(f(day)) }}
          />
        ))}
        <span
          aria-hidden
          className="pointer-events-none absolute top-[5px] size-4 -translate-x-1/2 rounded-pill bg-accent motion-tint shadow-[0_0_0_4px_var(--accent-soft)] peer-focus-visible:shadow-[0_0_0_4px_var(--focus-ring)]"
          style={{ left: at(here / last) }}
        />
        <span
          aria-hidden
          className="pointer-events-none absolute top-[24px] -translate-x-1/2 whitespace-nowrap text-[12.5px] font-semibold text-accent-text"
          style={{ left: at(here / last) }}
        >
          {value ? dayLabel(value) : ""}
        </span>
        {months.map((day) => (
          <span
            key={day}
            aria-hidden
            className="pointer-events-none absolute top-[24px] text-[12.5px] text-faint"
            style={{ left: at(f(day)) }}
          >
            {monthLabel(day)}
          </span>
        ))}
        <span aria-hidden className="pointer-events-none absolute right-0 top-[24px] text-[12.5px] text-faint">
          Today
        </span>
      </div>
    </div>
  );
}

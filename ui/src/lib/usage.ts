import { addDays, dayLabel, daysBetween, dayLong, today } from "@/components/insights/days";
import { formatLimit } from "@/lib/fields";
import { formatBytes } from "@/lib/format";
import type { StatsDay, StatsGap, StatsMetric, StatsSeries, StatsUnit } from "@/lib/queries/useStats";

/**
 * The arithmetic of statistics, kept out of the renderers: a value in its
 * unit's words, a makerspace day in words, which days of a series are gaps,
 * and add-on text capped. Nothing here knows a product or a kind; the unit is
 * the registry's, the rest is the descriptor.
 */

export const WINDOW_DAYS = 30;

/** A value in its unit: bytes in binary steps (as the storage card), the rest as a limit is said. */
export function formatValue(value: number, unit: StatsUnit): string {
  if (unit === "bytes") return formatBytes(value);
  if (unit === "count") return value.toLocaleString("en-GB", { maximumFractionDigits: 1 });
  if (unit === "seconds") {
    const [n, word] = value >= 3600 ? [value / 3600, "hour"] : value >= 60 ? [value / 60, "minute"] : [value, "second"];
    const shown = Math.round(n * 10) / 10;
    return `${shown} ${word}${shown === 1 ? "" : "s"}`;
  }
  return formatLimit(Number.isInteger(value) ? value : Math.round(value * 10) / 10, unit);
}

/** "9 Nov", and with its year when it is not this year's: a missing year a year away is a false statement. */
export function dayWords(day: string): string {
  return day.slice(0, 4) === today().slice(0, 4) ? dayLabel(day) : dayLong(day);
}

/** `end` is exclusive: "8–9 Nov" for start 8, end 10; "8 Nov" for one day; both ends named across months. */
export function gapRange(gap: Pick<StatsGap, "start" | "end">): string {
  const last = addDays(gap.end, -1);
  if (last <= gap.start) return dayWords(gap.start);
  const [a, b] = [dayWords(gap.start), dayWords(last)];
  const [da, ma] = a.split(" ");
  const [, mb] = b.split(" ");
  return ma === mb && gap.start.slice(0, 4) === last.slice(0, 4) ? `${da}–${b}` : `${a} to ${b}`;
}

/**
 * Add-on text is quoted, and cut at `cap` characters with an ellipsis: React
 * escapes it, this bounds it. Control and format characters (bidi overrides,
 * zero-width joiners) are removed, so add-on text cannot reorder or hide part
 * of Syndra's own line; a control character that was a space or a line break
 * stays a space.
 */
export function capText(text: string, cap: number): string {
  const chars = Array.from(text.replace(/\p{Cf}/gu, "").replace(/\p{Cc}/gu, " ").trim());
  return chars.length <= cap ? chars.join("") : `${chars.slice(0, cap - 1).join("").trimEnd()}…`;
}
export const quoted = (text: string, cap: number): string => `“${capText(text, cap)}”`;
export const CAP = { label: 60, value: 40, reason: 120 } as const;

/** A series' dimensions as words: `Share: “makers”`, in the descriptor's order of dimensions, the add-on's text quoted. */
export function dimensionWords(metric: Pick<StatsMetric, "dimensions">, dims: Record<string, string> | undefined): string {
  const parts: string[] = [];
  for (const d of metric.dimensions ?? []) {
    if (dims?.[d.name] !== undefined) parts.push(`${capText(d.label, CAP.label)}: ${quoted(dims[d.name], CAP.value)}`);
  }
  return parts.join(", ");
}

/** A gap belongs to a series when every key=value it names is the series' (the backend's rule; `{}` is the whole metric). */
export const gapIsOn = (gap: Pick<StatsGap, "dimensions">, series: Pick<StatsSeries, "dimensions">): boolean =>
  Object.entries(gap.dimensions ?? {}).every(([k, v]) => series.dimensions[k] === v);

/** The last `WINDOW_DAYS` makerspace days, oldest first, ending today. */
export function windowDays(end: string = today()): string[] {
  return Array.from({ length: WINDOW_DAYS }, (_, i) => addDays(end, i - (WINDOW_DAYS - 1)));
}

/** How many days a window starting on `from` covers, today included: N days are today − (N − 1) through today, as the backend keeps them. */
export const daysSince = (from: string, end: string = today()): number => daysBetween(from, end) + 1;

export interface Plot {
  /**
   * Every day of the window with what is known about it: a value, a gap (no
   * value, `kept` when it is before the day the window was read from), or neither (`pending`
   * when it is after the last final day, else a gauge's no new sample).
   */
  cells: Array<{ day: string; value?: number; gap: boolean; kept?: boolean; pending?: boolean }>;
  /** The days nobody could observe, as runs of cell indexes `[first, last]`, for the hatching. */
  gapRuns: Array<[number, number]>;
}

/**
 * One series over the window. A day inside a gap, or before `from` (the day
 * the backend read the window from, `window_30d_from`: later than the
 * window's first day only when retention keeps fewer, and the days before it
 * are gone, not zero), is a gap whatever the points say. Outside one, a counter's missing day is 0
 * only when it is final (not after `countedThrough`); a day not yet counted
 * has no value. That is the only place a zero is made, and a gap or a day not
 * yet counted never gets one.
 */
export function plot(
  series: StatsSeries,
  gaps: StatsGap[],
  instrument: StatsMetric["instrument"],
  days: string[] = windowDays(),
  known: { from?: string; countedThrough?: string | null } = {},
): Plot {
  const given = new Map<string, number>(series.days.map((d: StatsDay) => [d.day, d.value]));
  const mine = gaps.filter((g) => gapIsOn(g, series));
  const cells = days.map((day) => {
    const kept = known.from != null && day < known.from;
    const gap = kept || mine.some((g) => day >= g.start && day < g.end);
    if (gap) return { day, gap, ...(kept ? { kept } : {}), value: undefined };
    const point = given.get(day);
    const pending = point === undefined && known.countedThrough != null && day > known.countedThrough;
    return { day, gap, ...(pending ? { pending } : {}), value: point ?? (instrument === "counter" && !pending ? 0 : undefined) };
  });
  const gapRuns: Array<[number, number]> = [];
  cells.forEach((c, i) => {
    if (!c.gap) return;
    const run = gapRuns[gapRuns.length - 1];
    if (run && run[1] === i - 1) run[1] = i;
    else gapRuns.push([i, i]);
  });
  return { cells, gapRuns };
}

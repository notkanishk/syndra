"use client";

import { useState, type ReactNode } from "react";

import * as copy from "@/components/usage/copy";
import { Band, cellWords } from "@/components/usage/Band";
import { Crossfade } from "@/lib/motion";
import type { StatsMetric, StatsSeries, UsageWindows } from "@/lib/queries/useStats";
import { CAP, capText, change, dayWords, daysSince, formatValue, plot, seriesName, windowDays, type Plot } from "@/lib/usage";

/**
 * One metric, drawn from its descriptor and nothing else (A-23 §3): a section
 * of folder rows, each a name, its band and its figure on one baseline, the
 * figures one right-aligned column in the stat face. A counter's figure is its
 * sum over the window, a level's its latest value with its day and, under it,
 * its change over the record. Nothing here knows what the add-on measures.
 *
 * A metric's series are never added together: each is its own row. The most a
 * section shows is `MAX_ROWS` rows; the rest is said, not drawn. Each section
 * has one axis and one readout seat, fed by its bands.
 */
const MAX_ROWS = 12;

/**
 * One row (§3.2): from the card's tablet step, the 200 · band · 240 ledger, each seat 40px and
 * bottom-aligned so the name, the band's baseline and the figure share one line; under it one column,
 * name, figure, band, the figure left-aligned.
 */
function Row({ name, band, figure, className = "" }: { name: ReactNode; band?: ReactNode; figure: ReactNode; className?: string }) {
  return (
    <div data-row className={`grid grid-cols-1 items-start gap-y-1.5 py-3 @tablet:grid-cols-[200px_minmax(0,1fr)_240px] @tablet:gap-x-6 @tablet:gap-y-0 ${className}`}>
      <span className="flex min-w-0 items-end @tablet:min-h-10">{name}</span>
      <span className="order-3 min-w-0 @tablet:order-none">{band}</span>
      <span className="order-2 flex min-w-0 flex-col items-start gap-[3px] @tablet:order-none @tablet:items-end @tablet:text-right">{figure}</span>
    </div>
  );
}

/**
 * A window's start for this metric: the window's own first day, or later where the record or
 * retention starts later. Named from it: the window's own words, or "the last n days" counted to
 * the backend's today.
 */
function inWindow(value: string, own: string, began: string | undefined, windows: UsageWindows, words: (value: string) => string) {
  const start = began !== undefined && began > own ? began : own;
  return { start, said: start === windows.kept_from || start === began ? copy.inLastDays(value, daysSince(start, windows.today)) : words(value) };
}

/**
 * The figure's first line: the value in the stat face (StatCard's figure) and the rest of its
 * sentence in muted, one line of text with a real space, so it reads and copies as words. With no
 * value, the sentence alone.
 */
function Figure({ value, said, second }: { value?: string; said: string; second?: string | null }) {
  const rest = value && said.startsWith(`${value} `) ? said.slice(value.length + 1) : null;
  return (
    <>
      <span className="flex @tablet:min-h-10 items-end @tablet:justify-end">
        {rest === null ? (
          <span data-figure className="text-[14px] leading-[1.3] text-muted">
            {said}
          </span>
        ) : (
          <span data-figure className="block text-[13px] leading-none @tablet:text-right">
            <span className="font-display text-[24px] font-semibold leading-none text-ink">{value}</span>{" "}
            <span className="ml-[3px] text-muted">{rest}</span>
          </span>
        )}
      </span>
      {second && <span className="text-[13px] tabular-nums text-faint">{second}</span>}
    </>
  );
}

/** A level's change over the record, in words; within rounding (the two read the same) it is unchanged. */
export function changeWords(cells: Plot, unit: StatsMetric["unit"]): string | null {
  const c = change(cells);
  if (!c) return null;
  const [from, to] = [formatValue(c.first, unit), formatValue(c.last, unit)];
  const day = dayWords(c.from);
  if (from === to) return copy.changeNone(day);
  const difference = formatValue(Math.abs(c.last - c.first), unit);
  return c.last > c.first ? copy.changeUp(difference, day) : copy.changeDown(difference, day);
}

function figureFor(metric: StatsMetric, series: StatsSeries, cells: Plot, windows: UsageWindows, began: string | undefined): ReactNode {
  if (metric.instrument === "counter") {
    // Cut short by retention or by the record, a sum is of the days it covers and says so; both
    // windows starting on one day are one sum, said once.
    const a = inWindow(formatValue(series.sum_30d ?? 0, metric.unit), windows.window_30d_from, began, windows, copy.inDays);
    const b = inWindow(formatValue(series.sum_12m ?? 0, metric.unit), windows.window_12m_from, began, windows, copy.inMonths);
    return <Figure value={formatValue(series.sum_30d ?? 0, metric.unit)} said={a.said} second={b.start === a.start ? null : b.said} />;
  }
  if (!series.latest) return <Figure said={copy.NO_RECENT_VALUE} />;
  const value = formatValue(series.latest.value, metric.unit);
  return <Figure value={value} said={copy.latestOn(value, dayWords(series.latest.day))} second={changeWords(cells, metric.unit)} />;
}

/** A row with no band: the metric's label in the name seat, an empty band seat, a sentence in the figure seat. */
function LabelRow({ label, said }: { label: string; said: string }) {
  return <Row className="pt-0 pb-0" name={<h3 className="type-label text-label [overflow-wrap:anywhere]">{label}</h3>} figure={<Figure said={said} />} />;
}

/** The section's axis: the window's first day, and the readout seat (today at rest, else the day being read). */
export function Axis({ first, readout, inset = true }: { first: string; readout: string; inset?: boolean }) {
  return (
    <div data-axis className={`flex justify-between gap-3 text-[12.5px] tabular-nums leading-[1.3] text-faint ${inset ? "pt-1.5 @tablet:pl-[224px] @tablet:pr-[264px]" : "pt-0.5"}`}>
      <span>{dayWords(first)}</span>
      <span aria-live="polite" className="ml-auto text-right">
        <Crossfade id={readout} inline>
          {readout}
        </Crossfade>
      </span>
    </div>
  );
}

/**
 * `windows`: where the backend read this page's two windows from (a person's are cut short by
 * retention; a totals page's never are). The day the record began is the add-on's (`metric.began`,
 * its began gap). `first`/`titled`: the section's place in its card.
 */
export function MetricBlock({ metric, windows, first = false, titled = false }: { metric: StatsMetric; windows: UsageWindows; first?: boolean; titled?: boolean }) {
  const [read, setRead] = useState<{ row: number; index: number } | null>(null);
  const began = metric.began ?? undefined;
  const label = capText(metric.label, CAP.label);
  const rows = metric.watermark === null ? [] : metric.series.slice(0, MAX_ROWS);
  const days = windowDays(windows.today);
  const plots = rows.map((s) => plot(s, metric.gaps, metric.instrument, days, { from: windows.kept_from, began, countedThrough: metric.counted_through }));
  const names = rows.map((s) => seriesName(metric, s.dimensions));
  // The label goes on its own line when there are several rows, or a row has a name of its own; else it is the row's name.
  const many = rows.length > 1 || names.some(Boolean);
  const keptDays = windows.kept_from ? daysSince(windows.kept_from, windows.today) : undefined;
  const at = read && plots[read.row]?.cells[read.index];
  const readout = at ? (many ? copy.readoutNamed(names[read.row] || label, cellWords(at, metric.unit, keptDays)) : cellWords(at, metric.unit, keptDays)) : dayWords(windows.today);
  const quiet = [
    metric.collected?.unbound_accounts ? copy.unbound(metric.collected.unbound_accounts) : null,
    // A member's own card: what the backend left out because it names a person, said so the rest never reads as all of it.
    metric.watermark !== null && metric.left_out ? copy.leftOut(metric.left_out) : null,
  ].filter((s): s is string => s !== null);

  let body: ReactNode;
  if (metric.watermark === null) body = <LabelRow label={label} said={copy.NOT_COUNTED} />;
  else if (rows.length === 0) body = metric.left_out ? null : <LabelRow label={label} said={metric.instrument === "counter" ? copy.NOTHING_COUNTED : copy.NO_RECENT_VALUE} />;
  else
    body = (
      <>
        {rows.map((s, i) => (
          <Row
            key={JSON.stringify(s.dimensions)}
            className={`${i > 0 ? "row-divider" : "pt-0"} ${i === rows.length - 1 ? "pb-0" : ""}`}
            name={
              many ? (
                <span className="text-[15.5px] font-semibold leading-[1.25] [overflow-wrap:anywhere]">{names[i]}</span>
              ) : (
                <h3 className="type-label text-label [overflow-wrap:anywhere]">{label}</h3>
              )
            }
            band={
              <Band
                plot={plots[i]}
                instrument={metric.instrument}
                unit={metric.unit}
                keptDays={keptDays}
                what={names[i] ? `${label}, ${names[i]}` : label}
                readIndex={read?.row === i ? read.index : null}
                onRead={(index) => setRead(index === null ? null : { row: i, index })}
              />
            }
            figure={figureFor(metric, s, plots[i], windows, began)}
          />
        ))}
        <Axis first={days[0]} readout={readout} />
      </>
    );

  return (
    <section aria-label={label} className={`flex flex-col gap-3 px-5 py-[18px] ${first ? (titled ? "pt-1.5" : "") : "row-divider"}`}>
      {many && <h3 className="type-label text-label [overflow-wrap:anywhere]">{label}</h3>}
      {body && <div className="flex flex-col">{body}</div>}
      {metric.series.length > rows.length && rows.length > 0 && <span className="text-[13px] text-faint">{copy.noMoreLines(metric.series.length - rows.length)}</span>}
      {quiet.map((s) => (
        <span key={s} className="max-w-[80ch] text-[13px] text-faint [text-wrap:pretty]">
          {s}
        </span>
      ))}
    </section>
  );
}

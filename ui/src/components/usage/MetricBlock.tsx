import type { ReactNode } from "react";

import * as copy from "@/components/usage/copy";
import { Spark } from "@/components/usage/Spark";
import { CardRow } from "@/components/ui/Card";
import { Meter } from "@/components/ui/Meter";
import { formatRelative } from "@/lib/format";
import type { StatsMetric, StatsSeries, UsageWindows } from "@/lib/queries/useStats";
import { CAP, capText, dayWords, daysSince, dimensionWords, formatValue, gapRange, plot, quoted, type Plot } from "@/lib/usage";

/**
 * One metric, drawn from its descriptor and nothing else: a counter is its sum
 * over the window with a sparkline, a gauge its latest value with its day (a
 * meter when `against` names a limit that was read), a gap a footnote and a
 * hatched band. Nothing here knows what the add-on measures.
 *
 * A metric's series are never added together: each is its own row. The most a
 * card shows is `MAX_ROWS` rows and lines; the rest is said, not drawn.
 */
const MAX_ROWS = 12;

/** A person's observed limit per limit-field name, in the field's unit; absent is "not read", 0 is "no limit". */
export type Limits = Record<string, number | undefined>;

/** A gauge over the limit Syndra read: the one shared meter, or the sentence that there is none to draw. */
function Limit({ value, limit, metric }: { value: number; limit: number | undefined; metric: StatsMetric }) {
  if (limit === undefined) return <span className="text-[13px] text-faint">{copy.LIMIT_UNREAD}</span>;
  if (limit === 0) return <span className="text-[13px] text-muted">{copy.NO_LIMIT}</span>;
  const said = `${formatValue(value, metric.unit)} ${copy.ofLimit(formatValue(limit, metric.unit))}`;
  return (
    <span className="flex w-full max-w-[360px] flex-col gap-1.5">
      <Meter value={value} limit={limit} label={capText(metric.label, CAP.label)} valueText={said} />
      <span className="text-[13px] text-muted">{said}</span>
    </span>
  );
}

/**
 * A window's sum in words, named from the day the backend read it from: a
 * start retention set (`kept_from`) is "the last n days", n counted from it to
 * today; otherwise the window's own name.
 */
function inWindow(value: string, start: string, windows: UsageWindows, own: (value: string) => string): string {
  return start === windows.kept_from ? copy.inLastDays(value, daysSince(start)) : own(value);
}

function SeriesRow({ metric, series, cells, limits, windows }: { metric: StatsMetric; series: StatsSeries; cells: Plot; limits?: Limits; windows: UsageWindows }) {
  const where = dimensionWords(metric, series.dimensions);
  const counter = metric.instrument === "counter";
  const metered = limits !== undefined && !counter && metric.against !== undefined && series.latest !== undefined;

  let headline: ReactNode;
  let detail: ReactNode = null;
  if (counter) {
    // Kept for fewer days than a window: the sum is of the days kept, and says so.
    // Both windows starting on one day are one sum, said once.
    const { window_30d_from: w30, window_12m_from: w12 } = windows;
    headline = inWindow(formatValue(series.sum_30d ?? 0, metric.unit), w30, windows, copy.inDays);
    detail = w12 === w30 ? null : inWindow(formatValue(series.sum_12m ?? 0, metric.unit), w12, windows, copy.inMonths);
  } else if (series.latest) {
    headline = copy.latestOn(formatValue(series.latest.value, metric.unit), dayWords(series.latest.day));
  } else {
    headline = <span className="font-normal text-muted">{copy.NO_RECENT_VALUE}</span>;
  }

  return (
    <CardRow className="flex-wrap items-center gap-x-6 gap-y-2">
      <span className="flex min-w-0 flex-1 basis-[220px] flex-col gap-0.5">
        {where && <span className="text-[13.5px] font-semibold text-muted [overflow-wrap:anywhere]">{where}</span>}
        <span className="text-[15px] font-semibold tabular-nums">{headline}</span>
        {detail && <span className="text-[13px] tabular-nums text-faint">{detail}</span>}
        {metered && <Limit value={series.latest!.value} limit={limits![metric.against!]} metric={metric} />}
      </span>
      <Spark plot={cells} unit={metric.unit} keptDays={daysSince(windows.window_30d_from)} what={where ? `${capText(metric.label, CAP.label)}, ${where}` : capText(metric.label, CAP.label)} />
    </CardRow>
  );
}

/** `kept`: how many days Syndra keeps, said only when a drawn day is hatched for it. */
function Gaps({ metric, kept }: { metric: StatsMetric; kept?: number }) {
  if (metric.gaps.length === 0 && kept === undefined) return null;
  const shown = metric.gaps.slice(0, MAX_ROWS);
  return (
    <div className="row-divider flex flex-col gap-1 px-5 py-3 text-[13px] text-muted">
      <span className="font-semibold">{copy.GAPS_HEADING}</span>
      <ul className="flex flex-col gap-0.5">
        {kept !== undefined && <li>{copy.keptLine(kept)}</li>}
        {shown.map((g) => (
          <li key={`${g.start}-${g.end}-${JSON.stringify(g.dimensions ?? {})}`} className="[overflow-wrap:anywhere]">
            {copy.gapLine(gapRange(g), dimensionWords(metric, g.dimensions), quoted(g.reason, CAP.reason))}
          </li>
        ))}
        {metric.gaps.length > shown.length && <li className="text-faint">{copy.noMoreLines(metric.gaps.length - shown.length)}</li>}
      </ul>
      <span className="text-faint">{copy.GAPS_NOTE}</span>
    </div>
  );
}

/** The operator's collection lines on a totals page: how old the count is, what was left out, who is not a person Syndra manages. */
function Collected({ metric }: { metric: StatsMetric }) {
  const c = metric.collected;
  if (!c) return null;
  const dropped = Object.entries(c.dropped ?? {})
    .filter(([reason, n]) => n > 0 && copy.DROPPED[reason])
    .map(([reason, n]) => copy.DROPPED[reason](n));
  return (
    <div className="row-divider flex flex-col gap-0.5 px-5 py-3 text-[13px] text-faint">
      <span>{c.at ? copy.lastCounted(formatRelative(c.at)) : copy.NOT_COUNTED}</span>
      {c.unbound_accounts ? <span>{copy.unbound(c.unbound_accounts)}</span> : null}
      {dropped.length > 0 && <span>{copy.droppedLine(dropped)}</span>}
    </div>
  );
}

/** `limits` absent draws no meter (a totals page is no one person's); present and empty draws the meter that says its limit was not read. */
/** `windows`: where the backend read this page's two windows from (a person's are cut short by retention; a totals page's never are). */
export function MetricBlock({ metric, limits, windows }: { metric: StatsMetric; limits?: Limits; windows: UsageWindows }) {
  const rows = metric.series.slice(0, MAX_ROWS);
  const plots = (metric.watermark === null ? [] : rows).map((s) => plot(s, metric.gaps, metric.instrument, undefined, { from: windows.window_30d_from, countedThrough: metric.counted_through }));
  // The days before the window's start are hatched with Syndra's own reason, said only where one is drawn.
  const hatched = plots.some((p) => p.cells.some((c) => c.kept));
  return (
    <section aria-label={capText(metric.label, CAP.label)}>
      <div className="row-divider px-5 pb-1 pt-3.5">
        <h3 className="type-label text-label [overflow-wrap:anywhere]">{capText(metric.label, CAP.label)}</h3>
      </div>
      {metric.watermark === null ? (
        <CardRow>
          <span className="text-[14px] text-muted">{copy.NOT_COUNTED}</span>
        </CardRow>
      ) : rows.length === 0 ? (
        <CardRow>
          <span className="text-[14px] text-muted">{metric.instrument === "counter" ? copy.NOTHING_COUNTED : copy.NO_RECENT_VALUE}</span>
        </CardRow>
      ) : (
        rows.map((s, i) => <SeriesRow key={JSON.stringify(s.dimensions)} metric={metric} series={s} cells={plots[i]} limits={limits} windows={windows} />)
      )}
      {metric.series.length > rows.length && (
        <CardRow>
          <span className="text-[13px] text-faint">{copy.noMoreLines(metric.series.length - rows.length)}</span>
        </CardRow>
      )}
      <Gaps metric={metric} kept={hatched ? daysSince(windows.window_30d_from) : undefined} />
      <Collected metric={metric} />
    </section>
  );
}

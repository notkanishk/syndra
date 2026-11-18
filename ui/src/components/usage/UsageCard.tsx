"use client";

import { useState } from "react";

import * as copy from "@/components/usage/copy";
import { Band, cellWords } from "@/components/usage/Band";
import { insightsUsageHref } from "@/components/usage/InsightsUsage";
import { Axis, changeWords, MetricBlock } from "@/components/usage/MetricBlock";
import { meteredMetric, observedLimits } from "@/components/kinds/limits";
import { AdvancedOnly } from "@/components/ui/AdvancedOnly";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { addDays } from "@/components/insights/days";
import { formatRelative } from "@/lib/format";
import { Crossfade, Unfold } from "@/lib/motion";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { StatsMetric, UsageWindows } from "@/lib/queries/useStats";
import { CAP, capText, dayWords, daysSince, gapLines, gapRange, gapWhere, plot, quoted, windowDays } from "@/lib/usage";

/** The most lines the fold lists; the rest is said, not drawn. */
const MAX_LINES = 12;

/**
 * The card's one fold (A-23 Q2, closed): every gap in the widest window a metric shows, merged across
 * the card's metrics where range, series and reason are the same, then in Advanced what the last count
 * did not keep, then that a striped day is not a zero. A `began` gap is before the record, not a gap,
 * and is never listed. Present only when there is something to say.
 */
function Fold({ metrics, windows }: { metrics: StatsMetric[]; windows: UsageWindows }) {
  const [open, setOpen] = useState(false);
  const end = addDays(windows.today, 1);
  const seen = new Set<string>();
  const lines: Array<{ start: string; said: string }> = [];
  for (const metric of metrics) {
    if (metric.watermark === null) continue;
    // A counter's 12-month total is drawn, so every gap inside it is explained; a level shows 30 days.
    const from = metric.instrument === "counter" && windows.window_12m_from < windows.window_30d_from ? windows.window_12m_from : windows.window_30d_from;
    for (const g of gapLines(metric.gaps.filter((x) => x.kind !== "began"), from, end)) {
      const said = copy.gapLine(gapRange(g), gapWhere(metric, g), g.reason.trim() ? quoted(g.reason, CAP.reason) : copy.NO_REASON);
      if (seen.has(said)) continue;
      seen.add(said);
      lines.push({ start: g.start, said });
    }
  }
  lines.sort((a, b) => (a.start < b.start ? -1 : a.start > b.start ? 1 : 0));
  const dropped = metrics.flatMap((metric) => {
    const parts = Object.entries(metric.collected?.dropped ?? {})
      .filter(([reason, n]) => n > 0 && copy.DROPPED[reason])
      .map(([reason, n]) => copy.DROPPED[reason](n));
    return parts.length ? [copy.droppedFor(capText(metric.label, CAP.label), parts)] : [];
  });
  if (lines.length === 0 && dropped.length === 0) return null;
  const shown = lines.slice(0, MAX_LINES);

  const fold = (
    <div data-fold className="row-divider flex flex-col gap-2.5 px-5 py-3.5">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="inline-flex min-h-[var(--tap)] items-center gap-2.5 self-start text-left control press-scale tablet:min-h-0"
      >
        <FoldChevron open={open} />
        <span className="text-body font-semibold text-ink">{copy.FOLD_HEADING}</span>
      </button>
      <Unfold open={open}>
        <div className="flex measure flex-col gap-1 pl-[38px] text-small text-muted [overflow-wrap:anywhere]">
          {shown.length > 0 && (
            <ul aria-label={copy.GAPS_HEADING} className="flex flex-col gap-1">
              {shown.map((l) => (
                <li key={l.said}>{l.said}</li>
              ))}
              {lines.length > shown.length && <li className="text-faint">{copy.noMoreLines(lines.length - shown.length)}</li>}
            </ul>
          )}
          {dropped.length > 0 && (
            <AdvancedOnly gap={4}>
              <ul className="flex flex-col gap-1">
                {dropped.map((d) => (
                  <li key={d}>{d}</li>
                ))}
              </ul>
            </AdvancedOnly>
          )}
          <span className="text-faint">{copy.GAPS_NOTE}</span>
        </div>
      </Unfold>
    </div>
  );
  // Only the dropped values to say: a fold Basic does not have.
  return lines.length > 0 ? fold : <AdvancedOnly>{fold}</AdvancedOnly>;
}

/**
 * The card's end row: its next step left (on a totals card with per-person metrics, the one link to
 * Insights, Usage, in the words the system's kind gives, `step`), and right, faint, when the record
 * began inside the window and how old the count is (totals) or how long a person's numbers are kept.
 */
function End({ metrics, windows, target, step: insights }: { metrics: StatsMetric[]; windows: UsageWindows; target?: string; step?: string }) {
  const waiting = metrics.some((metric) => metric.watermark === null);
  const windowStart = windowDays(windows.today)[0];
  const began = metrics
    .map((metric) => metric.began)
    .filter((d): d is string => d != null && d > windowStart)
    .sort()[0];
  const counted = metrics
    .map((metric) => metric.collected?.at)
    .filter((at): at is string => Boolean(at))
    .sort()
    .pop();
  const { kept_from, window_30d_from: w30, window_12m_from: w12 } = windows;
  // Syndra's own reason: said whenever a window starts at what retention keeps.
  const kept = !waiting && kept_from !== undefined && (w30 === kept_from || w12 === kept_from);
  const right = [
    began ? copy.countingBegan(dayWords(began)) : null,
    target !== undefined && counted ? copy.lastCounted(formatRelative(counted)) : null,
    kept ? copy.keptLine(daysSince(kept_from, windows.today)) : null,
  ]
    .filter(Boolean)
    .join(" · ");
  // Insights, Usage holds per-person metrics only: a system with none has nothing there to open.
  const link = target !== undefined && !waiting && metrics.some((metric) => metric.per_subject) ? insightsUsageHref(target) : null;
  const step = waiting ? copy.NOT_COUNTED_NEXT : link ? (insights ?? copy.OVERVIEW_NEXT) : copy.NOTHING_TO_DO;
  return (
    // The link is the card's one action, in the row's seat; the seat is there with or without it, so the
    // faint facts never move with the data.
    <CardRow
      data-end
      seat="narrow"
      action={link ? <ArrowLink href={link}>{copy.OVERVIEW_NEXT_LINK}</ArrowLink> : undefined}
      className="text-small text-muted"
    >
      <span className="min-w-0 flex-1">{step}</span>
      {right && (
        <span className="text-faint">
          <Crossfade id={right} inline>
            {right}
          </Crossfade>
        </span>
      )}
    </CardRow>
  );
}

/**
 * The usage card (A-23 §3): "What I have used", the operator's "What {person} has used" (`title`), or
 * the target overview's totals (`target`, no title: the Region is the title; `step`, its kind's words
 * for the way to Insights). Its sections, one fold and one end row. It never draws a meter: a person's
 * limit has its meter, and its month, in the kind's card (R1, Q4).
 */
export function UsageCard({
  title,
  label,
  metrics,
  windows,
  target,
  step,
}: {
  title?: string;
  label?: string;
  metrics: StatsMetric[];
  windows: UsageWindows;
  target?: string;
  step?: string;
}) {
  // A system's own card (it names its target): a per-person figure opens its people on Insights' Usage (A-24 §10).
  const opens = target !== undefined ? insightsUsageHref(target) : undefined;
  return (
    <Card role="group" aria-label={title ?? label} className="@container">
      {title && <CardHeader title={title} />}
      {metrics.map((metric, i) => (
        <MetricBlock key={metric.name} metric={metric} windows={windows} first={i === 0} titled={Boolean(title)} opens={opens} />
      ))}
      <Fold metrics={metrics} windows={windows} />
      <End metrics={metrics} windows={windows} target={target} step={step} />
    </Card>
  );
}

/**
 * A system's usage card for one person's view, on the member's page (`person` absent) and on the
 * person page (the operator's). Where the add-on reports no statistics, or this person has no
 * per-person metric left to show, there is nothing to say and no card (the target overview says
 * "does not report" once, for the whole system). The metric the kind's card draws under its own meter
 * is left out, and only when that card draws it (`meteredMetric`, the kind's one answer): it is said
 * there (R1). A view with a kind whose usage is absent is Syndra's own store failing to answer: said,
 * with its step, and never as "no data".
 */
export function ViewUsage({ view, person }: { view: MyTargetView; person?: string }) {
  const usage = view.usage;
  if (!view.kind) return null;
  const title = person === undefined ? copy.MEMBER_TITLE : copy.operatorTitle(person);
  if (!usage) {
    return (
      <Card role="group" aria-label={title}>
        <CardHeader title={title} />
        <CardRow first>
          <span className="text-body text-muted">{copy.UNAVAILABLE}</span>
        </CardRow>
      </Card>
    );
  }
  if (!usage.reported) return null;
  const metered = meteredMetric(view, person !== undefined);
  const metrics = usage.metrics.filter((m) => m !== metered);
  if (metrics.length === 0) return null;
  return <UsageCard title={title} metrics={metrics} windows={usage} />;
}

/**
 * The home folder's month, under its meter in the storage cards (A-23 R1, §4): the level's change
 * over the record, its band against the limit Syndra read (none read, or 0 for "no limit": the band
 * on its own scale), and its own axis. Nothing when there is no such metric or nothing counted yet.
 * `operator`: the person page's card, else the member's own: the same question `ViewUsage` asks, so
 * the month is drawn here exactly when the usage card leaves it out.
 */
export function HomeMonth({ view, operator = false }: { view: MyTargetView; operator?: boolean }) {
  const [read, setRead] = useState<number | null>(null);
  const metric = meteredMetric(view, operator);
  const usage = view.usage;
  if (!metric || !usage?.reported) return null;
  const days = windowDays(usage.today);
  const p = plot(metric.series[0], metric.gaps, metric.instrument, days, { from: usage.kept_from, began: metric.began, countedThrough: metric.counted_through });
  const limit = observedLimits(view)[metric.against ?? ""];
  const keptDays = usage.kept_from ? daysSince(usage.kept_from, usage.today) : undefined;
  const changed = changeWords(p, metric.unit);
  return (
    <div data-home-month className="flex w-full max-w-[360px] flex-col gap-1">
      {changed && <span data-figure className="text-small text-faint">{changed}</span>}
      <Band
        plot={p}
        instrument={metric.instrument}
        unit={metric.unit}
        what={capText(metric.label, CAP.label)}
        keptDays={keptDays}
        limit={limit !== undefined && limit > 0 ? limit : undefined}
        readIndex={read}
        onRead={setRead}
      />
      <Axis first={days[0]} readout={read === null ? dayWords(usage.today) : cellWords(p.cells[read], metric.unit, keptDays)} inset={false} />
    </div>
  );
}

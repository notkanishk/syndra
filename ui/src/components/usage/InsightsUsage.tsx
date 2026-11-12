"use client";

import * as copy from "@/components/usage/copy";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { useInsightsStats, type InsightsStatsMetric, type StatsUnit } from "@/lib/queries/useStats";
import { CAP, capText, dayWords, dimensionWords, formatValue } from "@/lib/usage";

/**
 * Insights, Usage (Advanced view): every system's per-person metrics grouped
 * by unit, each as a spread over the people who used it (how many, the middle,
 * nine in ten) and never who. One person's numbers are on their own page.
 * Series are never added together: each breakdown is its own row, and the bar
 * is that row's own range (its middle inside its nine-in-ten), so two rows are
 * compared by their numbers and never by their bars.
 */
const UNITS: StatsUnit[] = ["count", "items", "bytes", "seconds", "grams"];

function Spread({ metric, row }: { metric: InsightsStatsMetric; row: InsightsStatsMetric["series"][number] }) {
  const where = dimensionWords(metric, row.dimensions);
  const p50 = formatValue(row.p50, metric.unit);
  const p90 = formatValue(row.p90, metric.unit);
  const share = row.p90 > 0 ? Math.min(100, (row.p50 / row.p90) * 100) : 0;
  return (
    <CardRow className="flex-wrap items-center gap-x-6 gap-y-2">
      <span className="flex min-w-0 flex-1 basis-[260px] flex-col gap-0.5">
        {where && <span className="text-[14.5px] font-semibold [overflow-wrap:anywhere]">{where}</span>}
        <span className="text-[13px] tabular-nums text-muted">
          {copy.members(row.members)}. {copy.spread(p50, p90)}
        </span>
      </span>
      {/* Zero to nine in ten; the darker part is up to the middle. Aria-hidden: the sentence beside it holds the numbers. */}
      <span aria-hidden className="flex h-2 w-[132px] shrink-0 overflow-hidden rounded-full bg-accent-soft">
        <span className="block h-full bg-accent" style={{ width: `${share}%` }} />
      </span>
    </CardRow>
  );
}

export function InsightsUsage() {
  const stats = useInsightsStats();
  const metrics = stats.data?.metrics ?? [];
  const groups = UNITS.map((unit) => ({ unit, metrics: metrics.filter((m) => m.unit === unit) })).filter((g) => g.metrics.length > 0);

  return (
    <>
      <p className="text-[13px] text-muted">{copy.INSIGHTS_LEDE}</p>
      {stats.data && (
        <>
          <span className="text-[13px] text-faint">{copy.windowFrom(dayWords(stats.data.from))}</span>
          <ReadFreshness
            subject={copy.INSIGHTS_SUBJECT}
            state={{ readAt: stats.dataUpdatedAt ? new Date(stats.dataUpdatedAt).toISOString() : null, current: stats.isError ? false : undefined }}
          />
        </>
      )}
      <ListStates
        isLoading={stats.isLoading}
        error={stats.error}
        onRetry={() => stats.refetch()}
        errorTitle={copy.INSIGHTS_ERROR}
        isEmpty={metrics.length === 0}
        skeleton={<RowSkeleton rows={4} avatar={false} label={copy.INSIGHTS_LOADING} />}
        empty={<EmptyState title={copy.INSIGHTS_EMPTY} />}
      >
        {groups.map((g) => (
          <Card key={g.unit}>
            <CardHeader title={copy.UNIT_HEADING[g.unit]} />
            {g.metrics.map((m) => (
              <section key={`${m.target}/${m.name}`} aria-label={`${capText(m.system, CAP.label)}: ${capText(m.label, CAP.label)}`}>
                <div className="row-divider px-5 pb-1 pt-3.5">
                  <h3 className="type-label text-label [overflow-wrap:anywhere]">
                    {capText(m.system, CAP.label)}: {capText(m.label, CAP.label)}
                  </h3>
                </div>
                {m.series.map((row) => (
                  <Spread key={JSON.stringify(row.dimensions)} metric={m} row={row} />
                ))}
              </section>
            ))}
          </Card>
        ))}
        <p className="flex flex-wrap items-center gap-x-3 text-[13px] text-muted">
          <span>{copy.INSIGHTS_NEXT}</span>
          <ArrowLink href="/users">{copy.INSIGHTS_NEXT_LINK}</ArrowLink>
        </p>
      </ListStates>
    </>
  );
}

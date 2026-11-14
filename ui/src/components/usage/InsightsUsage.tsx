"use client";

import * as copy from "@/components/usage/copy";
import { NameLink, nameHref } from "@/components/names/NameLink";
import { Unresolved, UNKNOWN_ACCOUNT } from "@/components/names/Unresolved";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardColumns, CardHeader, CardRow } from "@/components/ui/Card";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { useInsightsStats, type InsightsPerson, type InsightsSeries, type InsightsStatsMetric, type StatsUnit } from "@/lib/queries/useStats";
import { daysBetween } from "@/components/insights/days";
import { CAP, capText, dayWords, dimensionWords, formatValue } from "@/lib/usage";

/**
 * Insights, Usage (Advanced view, operators only): every system's per-person
 * metrics grouped by unit. Each breakdown is its spread over the people who
 * used it in the last 30 days (how many, the middle, nine in ten), with no
 * floor, and then those people and anyone else who used it in the last 12
 * months, by name, each linking to their page (the owner's ruling of 14 Nov
 * 2026: an operator can trace usage to a person). The people are in order of
 * the last 30 days, largest first, and the headings re-sort; nothing labels
 * anyone by their place in that order.
 *
 * Series are never added together: each breakdown is its own row, and the bar
 * is that row's own range (its middle inside its nine-in-ten), so two rows are
 * compared by their numbers and never by their bars.
 */
const UNITS: StatsUnit[] = ["count", "items", "bytes", "seconds", "grams"];

function Spread({ metric, row }: { metric: InsightsStatsMetric; row: InsightsSeries }) {
  const where = dimensionWords(metric, row.dimensions);
  const p50 = formatValue(row.p50, metric.unit);
  const p90 = formatValue(row.p90, metric.unit);
  const share = row.p90 > 0 ? Math.min(100, (row.p50 / row.p90) * 100) : 0;
  return (
    <CardRow className="flex-wrap items-center gap-x-6 gap-y-2">
      <span className="flex min-w-0 flex-1 basis-[260px] flex-col gap-0.5">
        {where && <span className="text-[14.5px] font-semibold [overflow-wrap:anywhere]">{where}</span>}
        <span className="text-[13px] tabular-nums text-muted">
          {row.members === 0 ? copy.NOBODY_RECENTLY : <>{copy.members(row.members)}. {copy.spread(p50, p90)}</>}
        </span>
      </span>
      {/* Zero to nine in ten; the darker part is up to the middle. Aria-hidden: the sentence beside it holds the numbers. */}
      {row.members > 0 && (
        <span aria-hidden className="flex h-2 w-[132px] shrink-0 overflow-hidden rounded-full bg-accent-soft">
          <span className="block h-full bg-accent" style={{ width: `${share}%` }} />
        </span>
      )}
    </CardRow>
  );
}

type PeopleSort = "person" | "last30" | "last12";

/** A gauge's missing 30 days sort after every value, whichever way the column is turned. */
const LAST = Number.MAX_VALUE;

/**
 * One breakdown's people: Person, Last 30 days, Last 12 months. A number's
 * first order is largest first, as Operations' time is newest first, and a
 * second press turns it; the name is the directory's, never the id, linking
 * to the person's page.
 */
function People({ metric, row, what }: { metric: InsightsStatsMetric; row: InsightsSeries; what: string }) {
  const [sort, setSort, view] = useSort<PeopleSort>("last30");
  const dir = view.sort.dir;
  const by: Record<PeopleSort, (p: InsightsPerson) => string | number> = {
    person: (p) => p.name || UNKNOWN_ACCOUNT,
    last30: (p) => (p.last_30d === null ? dir * LAST : -p.last_30d),
    last12: (p) => -p.last_12m,
  };
  const rows = orderBy(row.people, view.sort, by);
  const value = (v: number | null) => (v === null ? <span className="text-faint">{copy.NO_VALUE}</span> : formatValue(v, metric.unit));
  return (
    <div role="table" aria-label={copy.peopleLabel(what)}>
      <CardColumns role="row">
        <span className="min-w-0 flex-1">
          <SortHead label={copy.PERSON_COLUMN} name="person" sort={sort} onSort={setSort} />
        </span>
        <span className="w-[130px] shrink-0 text-right">
          <SortHead label={copy.LAST_30_COLUMN} name="last30" sort={sort} onSort={setSort} />
        </span>
        <span className="w-[130px] shrink-0 text-right">
          <SortHead label={copy.LAST_12_COLUMN} name="last12" sort={sort} onSort={setSort} />
        </span>
      </CardColumns>
      <div role="rowgroup" className={view.className}>
        {rows.map((p) => (
          <div key={p.user_id} role="row" className="row-divider flex flex-wrap items-baseline gap-x-[18px] gap-y-1 px-5 py-2 text-[14px] tablet:flex-nowrap">
            <span role="cell" className="w-full min-w-0 [overflow-wrap:anywhere] tablet:w-auto tablet:flex-1">
              {p.name ? (
                <NameLink href={nameHref.user(p.user_id)} className="font-semibold">{p.name}</NameLink>
              ) : (
                <Unresolved label={UNKNOWN_ACCOUNT} id={p.user_id} idLabel={copy.ACCOUNT_ID} href={nameHref.user(p.user_id)} />
              )}
            </span>
            <span role="cell" className="flex flex-col tabular-nums tablet:w-[130px] tablet:shrink-0 tablet:text-right">
              <span className="text-[12.5px] text-faint tablet:hidden">{copy.LAST_30_COLUMN}</span>
              {value(p.last_30d)}
            </span>
            <span role="cell" className="flex flex-col tabular-nums tablet:w-[130px] tablet:shrink-0 tablet:text-right">
              <span className="text-[12.5px] text-faint tablet:hidden">{copy.LAST_12_COLUMN}</span>
              {value(p.last_12m)}
            </span>
          </div>
        ))}
      </div>
    </div>
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
                {/* The backend started a window later than its own: the days it covers are the window's less those before. */}
                {stats.data && m.window_30d_from > stats.data.from && (
                  <p className="px-5 pb-1 text-[13px] text-faint">{copy.spreadDays(stats.data.window_days - daysBetween(stats.data.from, m.window_30d_from))}</p>
                )}
                {stats.data && m.window_12m_from > stats.data.from_12m && (
                  <p className="px-5 pb-1 text-[13px] text-faint">{copy.yearDays(daysBetween(m.window_12m_from, stats.data.from) + stats.data.window_days)}</p>
                )}
                {m.series.map((row) => {
                  const what = [`${capText(m.system, CAP.label)}: ${capText(m.label, CAP.label)}`, dimensionWords(m, row.dimensions)].filter(Boolean).join(", ");
                  return (
                    <div key={JSON.stringify(row.dimensions)}>
                      <Spread metric={m} row={row} />
                      <People metric={m} row={row} what={what} />
                    </div>
                  );
                })}
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

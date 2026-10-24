"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";

import { buildModel } from "@/components/insights/model";
import { projectLabel } from "@/components/names/Unresolved";
import type { InsightsUrl } from "@/components/insights/useInsightsUrl";
import { ListStates, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { Segmented } from "@/components/ui/Select";
import { useInsightsTrends, useInsightsUsage, type InsightsTrends, type TrendPeriod } from "@/lib/queries/useInsights";
import { useTopology } from "@/lib/queries/useTopology";
import { formatShortDate } from "@/lib/format";

const PERIODS: Array<{ value: TrendPeriod; label: string }> = [
  { value: "30d", label: "30 days" },
  { value: "term", label: "This term" },
  { value: "year", label: "12 months" },
];

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
const people = (n: number) => plural(n, "person", "people");

function hours(h: number | null): string {
  if (h == null) return "No decisions yet";
  if (h < 1) return "Under an hour";
  if (h < 48) return plural(Math.round(h), "hour", "hours");
  return plural(Math.round(h / 24), "day", "days");
}

function minutes(m: number): string {
  if (m < 1) return "Under a minute";
  if (m < 90) return plural(Math.round(m), "minute", "minutes");
  return hours(m / 60);
}

/** One fact: what it is, then its number, on a hairline. */
function Fact({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="row-divider flex items-baseline justify-between gap-4 px-5 py-3 text-[13.5px]">
      <span className="text-muted">{label}</span>
      <span className="font-semibold tabular-nums">{value}</span>
    </div>
  );
}

const ENDING_ROW = "row-divider grid grid-cols-[1fr_72px_72px] items-baseline gap-4 px-5 py-3";

const Quiet = ({ children }: { children: ReactNode }) => (
  <p className="row-divider px-5 py-3.5 text-[13.5px] text-muted [text-wrap:pretty]">{children}</p>
);

/** Weeks as columns: given in the accent, revoked beneath in warn. The words carry it; the bars show the shape. */
function Weeks({ weekly }: { weekly: InsightsTrends["weekly"] }) {
  const given = (w: InsightsTrends["weekly"][number]) => w.through_bundle + w.by_request + w.by_hand;
  const top = Math.max(1, ...weekly.map((w) => Math.max(given(w), w.taken_away)));
  if (weekly.length === 0) return <Quiet>No weeks in this period yet.</Quiet>;
  return (
    <div aria-hidden className="row-divider flex h-24 items-end gap-1 px-5 pb-3 pt-4">
      {weekly.map((w) => (
        <div key={w.week_start} className="flex h-full min-w-0 flex-1 flex-col justify-end gap-0.5">
          <span className="rounded-[2px] bg-accent" style={{ height: `${(given(w) / top) * 100}%`, minHeight: given(w) ? 2 : 0 }} />
          <span className="rounded-[2px] bg-warn" style={{ height: `${(w.taken_away / top) * 40}%`, minHeight: w.taken_away ? 2 : 0 }} />
        </div>
      ))}
    </div>
  );
}

export function TrendsTab({ url }: { url: InsightsUrl }) {
  const trends = useInsightsTrends(url.period);
  const topology = useTopology();
  const usage = useInsightsUsage().data;
  const model = useMemo(() => (topology.data ? buildModel(topology.data) : null), [topology.data]);
  const t = trends.data;
  const roleName = (projectId: string, roleKey: string) => {
    const id = model?.roleId(projectId, roleKey);
    return id ? model!.nameOf(id) : "A role no longer on the map";
  };

  return (
    <>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <Segmented
          label="Period"
          options={PERIODS}
          value={url.period}
          onChange={(p) => url.set({ period: p === "term" ? null : p })}
        />
        {/* The term line: which term this is reading, then the one place its dates are set. */}
        {url.period === "term" && t ? (
          <>
            <span className="text-[13px] text-faint">
              This term is {t.window.label}, {formatShortDate(`${t.window.from}T12:00:00`)} to{" "}
              {formatShortDate(`${t.window.to}T12:00:00`)}
            </span>
            <ArrowLink href="/settings">Set in Makerspace settings</ArrowLink>
          </>
        ) : (
          <span className="text-[13px] text-faint">{t?.window.label ?? ""}</span>
        )}
      </div>

      <ListStates
        isLoading={trends.isLoading}
        error={t ? null : trends.error}
        isEmpty={false}
        onRetry={() => trends.refetch()}
        errorTitle="Couldn't load the trends."
        skeleton={<RowSkeleton rows={4} avatar={false} label="Loading trends" />}
        empty={null}
      >
        {t && (
          <div className="grid gap-4 tablet:grid-cols-2">
            <Card className="tablet:col-span-2">
              <CardHeader title="Access given and revoked, by week" note={`${t.totals.given} given, ${t.totals.taken_away} revoked`} />
              <Weeks weekly={t.weekly} />
              <Fact label="Through a bundle" value={t.weekly.reduce((n, w) => n + w.through_bundle, 0)} />
              <Fact label="By request" value={t.weekly.reduce((n, w) => n + w.by_request, 0)} />
              <Fact label="By hand" value={t.weekly.reduce((n, w) => n + w.by_hand, 0)} />
              <Fact label="Revoked" value={t.totals.taken_away} />
            </Card>

            <Card>
              <CardHeader title="Requests" />
              <Fact label="Came in" value={t.requests.came_in} />
              <Fact label="Approved" value={t.requests.approved} />
              <Fact label="Declined" value={t.requests.declined} />
              <Fact label="Withdrawn" value={t.requests.withdrawn} />
              <Fact label="Usual time to decide" value={hours(t.requests.median_hours_to_decide)} />
              {t.requests.declined_reasons.map((r) => (
                <Fact key={r.reason} label={`Declined: ${r.reason}`} value={r.n} />
              ))}
              {t.requests.outside_bundles.length === 0 ? (
                <Quiet>Nobody has asked for a role that no bundle gives.</Quiet>
              ) : (
                t.requests.outside_bundles.map((r) => (
                  <Fact key={`${r.project_id}:${r.role_key}`} label={`Asked for outside a bundle: ${roleName(r.project_id, r.role_key)}`} value={r.n} />
                ))
              )}
            </Card>

            <Card>
              <CardHeader title="Delivery" />
              {t.sending.sample === 0 || t.sending.median_minutes_approval_to_sent == null ? (
                <Quiet>Delivery history starts being kept from now.</Quiet>
              ) : (
                <Fact
                  label={`Usual time from approval to sent, over ${plural(t.sending.sample, "send", "sends")}`}
                  value={minutes(t.sending.median_minutes_approval_to_sent)}
                />
              )}
            </Card>

            <Card>
              <CardHeader title="Drift" />
              <Fact label="Found" value={t.drift.found} />
              <Fact label="Cleared" value={t.drift.cleared} />
              <Fact
                label="Usual time to clear"
                value={t.drift.median_hours_to_clear == null ? "Nothing cleared yet" : hours(t.drift.median_hours_to_clear)}
              />
            </Card>

            <Card>
              <CardHeader title="Ending soon" note="People whose direct access ends within 14 and 30 days. Each number opens Expiring access in the same window." />
              {t.ending_soon.length === 0 ? (
                <Quiet>No direct access ends in the next 30 days.</Quiet>
              ) : (
                // The product's one window (A-04 §4). In order by the 30-day count, most first,
                // and the arrow says so (A-07 §17).
                <div role="table" aria-label="Ending soon, by project">
                  <div role="row" className={`${ENDING_ROW} type-label`}>
                    <span role="columnheader">Project</span>
                    <span role="columnheader" className="text-right">
                      14 days
                    </span>
                    <span role="columnheader" className="text-right text-ink">
                      30 days<span aria-hidden> ↑</span>
                    </span>
                  </div>
                  {[...t.ending_soon]
                    .sort((a, b) => b.d30 - a.d30)
                    .map((p) => (
                      <div key={p.project_id} role="row" className={`${ENDING_ROW} text-[13.5px]`}>
                        <span role="cell" className="text-muted">
                          {projectLabel(p.project_name, p.project_id)}
                        </span>
                        <span role="cell" className="text-right font-semibold tabular-nums">
                          <EndingLink within={14} n={p.d14} />
                        </span>
                        <span role="cell" className="text-right font-semibold tabular-nums">
                          <EndingLink within={30} n={p.d30} />
                        </span>
                      </div>
                    ))}
                </div>
              )}
            </Card>

            <Card>
              <CardHeader
                title="Held but not used"
                note={usage ? `No sign-in to its app in ${usage.window_days} days.` : undefined}
                action={<CardHeaderLink href="/insights?lens=use">Show on the map</CardHeaderLink>}
              />
              {!usage ? (
                <Quiet>Reading recent use.</Quiet>
              ) : !usage.sources.sign_in.receiving ? (
                <Quiet>This fills in once sign-ins arrive.</Quiet>
              ) : !usage.holders_observed ? (
                <Quiet>Syndra hasn&apos;t read who holds what from Zitadel yet.</Quiet>
              ) : usage.held_not_used.length === 0 ? (
                <Quiet>Every role someone holds has been used in {usage.window_days} days.</Quiet>
              ) : (
                [...usage.held_not_used]
                  .sort((a, b) => b.people - a.people)
                  .map((r) => <Fact key={`${r.project_id}:${r.role_key}`} label={roleName(r.project_id, r.role_key)} value={people(r.people)} />)
              )}
            </Card>

            <Card>
              <CardHeader title="Upkeep" />
              <Fact label="People who left and still hold access" value={t.upkeep.people_who_left_holding} />
              <Fact label="Drift waiting" value={t.upkeep.drift_open} />
              <Fact label="Roles nobody holds" value={t.upkeep.roles_nobody_has} />
              <Fact label="Bundles with nothing new to give" value={t.upkeep.bundles_nothing_new} />
              <Fact label="Overlapping bundles" value={t.upkeep.overlaps} />
            </Card>
          </div>
        )}
      </ListStates>
    </>
  );
}

/** Each Ending soon count opens Expiring access in the same window (route-insights, 23 Oct). */
function EndingLink({ within, n }: { within: 14 | 30; n: number }) {
  return (
    <Link
      href={`/review/expiring-access?within=${within}`}
      aria-label={`${n} in ${within} days: open Expiring access`}
      className="motion-tint hover:text-accent-text"
    >
      {n}
    </Link>
  );
}

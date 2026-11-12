"use client";

import * as copy from "@/components/usage/copy";
import { MetricBlock, type Limits } from "@/components/usage/MetricBlock";
import { observedLimits } from "@/components/kinds/limits";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { StatsMetric, UsageWindows } from "@/lib/queries/useStats";

/** Where a card ends: its next step, which is "Nothing for you to do." unless something waits on Syndra's next count. */
function End({ metrics }: { metrics: StatsMetric[] }) {
  return (
    <CardRow>
      <span className="text-[13px] text-muted">{metrics.some((m) => m.watermark === null) ? copy.NOT_COUNTED_NEXT : copy.NOTHING_TO_DO}</span>
    </CardRow>
  );
}

/** "What I have used", or the operator's "What {person} has used": the metrics the backend sent for this one person, drawn from their descriptors. */
export function UsageCard({ title, metrics, limits, windows }: { title: string; metrics: StatsMetric[]; limits?: Limits; windows: UsageWindows }) {
  return (
    <Card role="group" aria-label={title}>
      <CardHeader title={title} />
      {metrics.map((m) => (
        <MetricBlock key={m.name} metric={m} limits={limits} windows={windows} />
      ))}
      <End metrics={metrics} />
    </Card>
  );
}

/**
 * A system's usage card for one person's view, on the member's page (`person`
 * absent) and on the person page (the operator's). Where the add-on reports no
 * statistics, or this person has no per-person metric, there is nothing to say
 * and no card (the target overview says "does not report" once, for the whole
 * system). A view with a kind whose usage is absent is Syndra's own store
 * failing to answer: said, with its step, and never as "no data".
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
          <span className="text-[14px] text-muted">{copy.UNAVAILABLE}</span>
        </CardRow>
      </Card>
    );
  }
  if (!usage.reported || usage.metrics.length === 0) return null;
  return <UsageCard title={title} metrics={usage.metrics} limits={observedLimits(view)} windows={usage} />;
}

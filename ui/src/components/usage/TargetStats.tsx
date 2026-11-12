"use client";

import * as copy from "@/components/usage/copy";
import { MetricBlock } from "@/components/usage/MetricBlock";
import { RowSkeleton } from "@/components/states";
import { Card, CardRow } from "@/components/ui/Card";
import { useTargetStats } from "@/lib/queries/useStats";

/**
 * The target overview's statistics (operators): every metric the add-on
 * reports, as totals for everyone who used it, one row per breakdown, gaps
 * hatched, then the unbound-accounts and collection lines. Drawn from the
 * descriptors; "this add-on does not report statistics" when the feature is
 * absent, which is not the same sentence as "no data".
 */
export function TargetStats({ target }: { target: string }) {
  const stats = useTargetStats(target);

  if (stats.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={3} avatar={false} label={copy.OVERVIEW_TITLE} />
      </Card>
    );
  }
  if (stats.error || !stats.data) {
    return (
      <Card>
        <CardRow first>
          <span className="text-[14px] text-muted">{copy.UNAVAILABLE}</span>
        </CardRow>
      </Card>
    );
  }
  const { usage } = stats.data;
  if (!usage.reported) {
    return (
      <Card>
        <CardRow first>
          <span className="text-[14px] text-muted">{copy.ABSENT}</span>
        </CardRow>
      </Card>
    );
  }
  return (
    <Card>
      {usage.metrics.map((m) => (
        <MetricBlock key={m.name} metric={m} />
      ))}
      <CardRow first={usage.metrics.length === 0}>
        <span className="text-[13px] text-muted">{copy.NOTHING_TO_DO}</span>
      </CardRow>
    </Card>
  );
}

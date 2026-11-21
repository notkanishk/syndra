"use client";

import * as copy from "@/components/usage/copy";
import { UsageCard } from "@/components/usage/UsageCard";
import { usageStep } from "@/components/kinds/limits";
import { RowSkeleton } from "@/components/states";
import { Card, CardRow } from "@/components/ui/Card";
import { useTargetStats } from "@/lib/queries/useStats";

/**
 * The target overview's statistics (operators): every metric the add-on
 * reports, as totals for everyone who used it, one row per breakdown, in the
 * usage card (A-23), with the one link to Insights, Usage and how old the
 * count is. Drawn from the descriptors; "this add-on does not report
 * statistics" when the feature is absent, which is not the same sentence as
 * "no data".
 */
/** `name`: the system's name, which words why the newest days are not counted while it is down. */
export function TargetStats({ target, kind, name }: { target: string; kind?: string; name?: string }) {
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
          <span className="text-body text-muted">{copy.UNAVAILABLE}</span>
        </CardRow>
      </Card>
    );
  }
  const { usage } = stats.data;
  if (!usage.reported) {
    return (
      <Card>
        <CardRow first>
          <span className="text-body text-muted">{copy.ABSENT}</span>
        </CardRow>
      </Card>
    );
  }
  return <UsageCard label={copy.OVERVIEW_TITLE} metrics={usage.metrics} windows={usage} target={target} step={usageStep(kind)} system={name} />;
}

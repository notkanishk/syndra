"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";

import { Card } from "@/components/ui/Card";
import { TraceCell } from "@/components/audit/TraceCell";
import { actedOn, describeAction, groupByDay, machineName } from "@/lib/audit-vocabulary";
import { formatClock, formatShortDate } from "@/lib/format";
import { useAuditEntries, type AuditEntry } from "@/lib/queries/useAudit";

const PAGE = 50;

/**
 * One person's history — what they did, and what was done to them.
 *
 * Both directions, in one feed. A grant made *to* somebody is as much a part of
 * their record as one they made, and splitting the two would leave the most
 * common question ("who gave them this?") answered on neither tab. The rows say
 * which direction each entry runs so the person is never implied to have acted
 * when they were acted upon.
 *
 * Filtered server-side. Client-filtering the global tail would truncate
 * silently: an account whose last activity fell outside the most recent 200
 * rows would render an empty feed, which reads as "nothing ever happened" and
 * is a different claim entirely.
 */
export function PersonActivity({ userId, name }: { userId: string; name: string }) {
  const [limit, setLimit] = useState(PAGE);
  const entries = useAuditEntries({ userId, limit });

  const rows = useMemo(() => entries.data ?? [], [entries.data]);
  const days = useMemo(() => groupByDay(rows), [rows]);
  // The backend caps at 200. Past that the feed is genuinely partial, and
  // saying so beats a "Load more" button that quietly stops working.
  const atCap = rows.length >= 200;

  return (
    <Card>
      <ListStates
        isLoading={entries.isLoading}
        error={entries.error}
        isEmpty={rows.length === 0}
        onRetry={() => entries.refetch()}
        errorTitle="Couldn't load this person's activity."
        skeleton={<RowSkeleton rows={5} avatar={false} label="Loading activity" />}
        empty={
          <EmptyState
            title={`Nothing recorded for ${name} yet.`}
            guidance="Grants, revokes, bundle changes and request decisions involving this person are written here as they happen."
          />
        }
      >
        {days.map((group) => (
          <div key={group.day}>
            <div className="row-divider bg-tint-1 px-5 py-2 text-[12.5px] font-semibold text-muted">
              {formatShortDate(group.day)}
            </div>
            {group.entries.map((entry) => (
              <ActivityRow key={entry.id} entry={entry} userId={userId} />
            ))}
          </div>
        ))}

        {rows.length >= limit && !atCap && (
          <div className="row-divider flex items-center gap-4 px-5 py-3.5">
            <button
              type="button"
              onClick={() => setLimit((current) => Math.min(current + PAGE, 200))}
              className="rounded-pill border border-line-strong px-4 py-1.5 text-[13.5px] font-semibold motion-tint hover:bg-[var(--hover)]"
            >
              Load more
            </button>
          </div>
        )}
        {atCap && (
          <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
            Showing the most recent 200 entries — this is not their whole history. The full log is
            in{" "}
            <Link href="/audit" className="font-semibold text-accent-text">
              Review › Audit
            </Link>
            .
          </div>
        )}
      </ListStates>
    </Card>
  );
}

function ActivityRow({ entry, userId }: { entry: AuditEntry; userId: string }) {
  const { verb, destructive } = describeAction(entry.action);
  const direction = actedOn(entry, userId);

  return (
    <div className="row-divider flex flex-wrap items-baseline gap-4 px-5 py-3">
      <span className="w-[64px] shrink-0 text-[12.5px] text-faint">
        {formatClock(entry.created_at)}
      </span>

      <span className="min-w-[240px] flex-1 text-[14px]">
        <span className={destructive ? "font-semibold text-danger-text" : "font-semibold"}>
          {verb}
        </span>
        {/* The subject line changes with direction, because "Granted direct
            access" beside somebody's name means two opposite things depending
            on whether they were the hand or the recipient. */}
        {direction === "affected" ? (
          <span className="text-muted">
            {" — by "}
            <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
          </span>
        ) : direction === "acted" && entry.target_id && entry.target_id !== "-" ? (
          <span className="text-muted">
            {" — to "}
            <UserName id={entry.target_id} />
          </span>
        ) : (
          <span className="text-muted"> — by themselves</span>
        )}
      </span>

      <TraceCell entry={entry} className="w-[86px] shrink-0 text-right" />
    </div>
  );
}

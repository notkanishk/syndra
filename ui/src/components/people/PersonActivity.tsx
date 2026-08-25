"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";

import { Card } from "@/components/ui/Card";
import { TraceCell } from "@/components/audit/TraceCell";
import { actedOn, describeAction, groupByDay, machineName } from "@/lib/audit-vocabulary";
import { formatClock, formatList, formatShortDate } from "@/lib/format";
import { useAuditEntries, type AuditEntry } from "@/lib/queries/useAudit";
import { useTargets } from "@/lib/queries/useTargets";
import { useTargetActivity, type TargetActivityEvent } from "@/lib/queries/useTargetActivity";

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
    <div className="flex flex-col gap-[18px]">
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
              className="min-h-[44px] rounded-pill border border-line-strong px-4 text-[13.5px] font-semibold motion-tint hover:bg-[var(--hover)] desktop:min-h-0 desktop:py-1.5"
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

    <OnTheTargets userId={userId} name={name} />
    </div>
  );
}

/**
 * What the TARGETS say, kept apart from what Syndra says.
 *
 * Two sources, two cards, deliberately. Syndra's feed above records what Syndra
 * did; this records what the account did on the target — including everything
 * that happened with no involvement from Syndra at all, which is exactly the
 * category a merged feed would hide. The reason a target is reconciled rather
 * than trusted is that it moves on its own, and a surface that interleaves the
 * two implies it does not.
 *
 * `activity.get` was implemented by the add-on and declared by the backend from
 * the day the platform landed, and nothing ever called it.
 */
function OnTheTargets({ userId, name }: { userId: string; name: string }) {
  const targets = useTargets();
  const rows = targets.data ?? [];
  if (rows.length === 0) return null;

  return (
    <>
      {rows.map((entry) => (
        <TargetActivityCard key={entry.target} target={entry.target} userId={userId} name={name} />
      ))}
    </>
  );
}

function TargetActivityCard({
  target,
  userId,
  name,
}: {
  target: string;
  userId: string;
  name: string;
}) {
  const activity = useTargetActivity(target, userId);
  const data = activity.data;

  // Nothing at all rather than an empty card: a person with no account on a
  // target has no activity to be missing, and a card saying so on every target
  // in the deployment is noise on the common case.
  if (activity.isError) return null;

  const events = data?.readable ? (data.events ?? []) : [];
  const uncovered = data?.uncovered_shares ?? [];

  return (
    <Card>
      <div className="row-divider flex flex-wrap items-baseline gap-2 px-5 py-3">
        <span className="text-[14.5px] font-semibold">On {target}</span>
        <span className="text-[13px] text-muted">
          Read from the target&rsquo;s own audit log, not from Syndra&rsquo;s record.
        </span>
      </div>

      <ListStates
        isLoading={activity.isLoading}
        error={null}
        errorTitle={`Couldn't read ${target}'s audit log.`}
        isEmpty={data?.readable === true && events.length === 0}
        skeleton={<RowSkeleton rows={3} avatar={false} label={`Reading ${target}`} />}
        empty={
          <EmptyState
            title={`No recorded activity for ${name} on ${target}.`}
            guidance={
              uncovered.length > 0
                ? "Some shares were not auditing this account, so this is not the same as nothing having happened — see below."
                : "The target's audit log has nothing for this account."
            }
          />
        }
      >
        {events.map((event, at) => (
          <TargetEventRow key={`${event.at}-${at}`} event={event} />
        ))}
      </ListStates>

      {/* The distinction the whole card exists for. "Could not look" and
          "nothing happened" are the same empty list otherwise, and they are
          opposite answers. */}
      {data && !data.readable && (
        <div className="border-t border-line px-5 py-3 text-[13px] text-warn-text">
          The target&rsquo;s audit log could not be read, so this is not a claim that nothing
          happened.{data.detail ? ` ${data.detail}` : ""}
        </div>
      )}

      {uncovered.length > 0 && (
        <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
          {/* Not "auditing is off". A share can have auditing switched on and
              still record nothing for this person, because TrueNAS scopes it by
              group — so the honest claim is about coverage of THIS account, and
              an operator sent to switch on a setting that is already on would
              find nothing wrong and conclude the gap was imaginary. */}
          Auditing on {formatList(uncovered)} does not cover {name}, so nothing on{" "}
          {uncovered.length === 1 ? "that share" : "those shares"} appears here whether or not it
          happened.
        </div>
      )}
    </Card>
  );
}

function TargetEventRow({ event }: { event: TargetActivityEvent }) {
  return (
    <div className="row-divider flex flex-wrap items-baseline gap-4 px-5 py-3">
      <span className="w-[64px] shrink-0 text-[12.5px] text-faint">{formatClock(event.at)}</span>
      <span className="min-w-[200px] flex-1 text-[14px]">
        <span className="font-semibold">{event.event}</span>
        {event.share ? <span className="text-muted"> — {event.share}</span> : null}
        {/* The reason, in the target's own vocabulary rather than a
            translation of it. `NT_STATUS_NO_SUCH_USER` is the string an
            operator searches for, and inventing friendlier wording would make
            it unfindable. */}
        {event.detail ? (
          <span className="ml-2 font-mono text-[12.5px] text-faint">{event.detail}</span>
        ) : null}
      </span>
      {/* Where from. Without it a week of refusals is a week of refusals from
          nowhere: on the live target 553 rows shared one verb and one outcome,
          and the only thing that distinguished them was the address. */}
      {event.address ? (
        <span className="shrink-0 font-mono text-[12.5px] text-faint">{event.address}</span>
      ) : null}
      {/* Refusals are the rows worth finding. An access that was denied is a
          different fact from one that never happened. */}
      {!event.success && (
        <span className="rounded-pill bg-warn-soft px-2.5 py-0.5 text-[12.5px] font-semibold text-warn-text">
          Refused
        </span>
      )}
    </div>
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

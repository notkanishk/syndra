"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";

import { useMemo, useState } from "react";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";

import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { SignInLookup } from "@/components/people/SignInLookup";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { TraceCell } from "@/components/audit/TraceCell";
import { actedOn, describeAction, machineName } from "@/lib/audit-vocabulary";
import { EventSentence, describeSends, isPasswordSend } from "@/components/audit/EventLine";
import { ActivityRows } from "@/components/results/ActivityList";
import { UNCOVERED_GUIDANCE as STORAGE_GUIDANCE, UncoveredNote } from "@/components/kinds/storage/ActivityCoverage";
import { UNCOVERED_GUIDANCE, uncoveredNote } from "@/components/results/copy";
import { formatClock, formatShortDate } from "@/lib/format";
import { isStorage } from "@/lib/nav";
import { sortNames } from "@/lib/sort";
import { useAuditEntries, type AuditEntry } from "@/lib/queries/useAudit";
import { useTargets } from "@/lib/queries/useTargets";
import { useTargetActivity, useTargetActivityAgain } from "@/lib/queries/useTargetActivity";
import { ReadFreshness, observed, type ReadState } from "@/components/ui/ReadFreshness";
import { STALE } from "@/lib/queries/fresh";

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
  const paging = usePaging(undefined, undefined, "activity");
  const limit = paging.limit;
  const entries = useAuditEntries({ userId, limit });

  // One row per change: the page's automatic resend is told inside the change
  // it repeats, never as a second change (A-13 §4). Audit keeps both rows.
  const rows = useMemo(() => (entries.data ?? []).filter((e) => e.operation?.attempt !== 2), [entries.data]);
  // Keyed by `formatShortDate`, the SAME makerspace-zone source the row's own
  // clock (`formatClock`) reads against. A key of `created_at.slice(0, 10)`
  // (the UTC date) once put an entry a few hours either side of UTC midnight
  // under yesterday's heading while its own clock read 04:18 the next day.
  const days = useMemo(() => groupByLocalDay(rows), [rows]);
  // The backend caps at 200. Past that the feed is genuinely partial, and
  // saying so beats a "Load more" button that quietly stops working.
  const read = entries.data?.length ?? 0;
  const atCap = read >= 200;
  const [lookingUp, setLookingUp] = useState(false);

  return (
    <div className="flex flex-col gap-4">
    {/* Sign-ins aren't history Syndra keeps (A-10 §6); they're read on demand
        from Zitadel's own log, with a reason (owner, 3 Nov). */}
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-5">
      <p className="m-0 min-w-0 flex-1 basis-[280px] text-[13.5px] leading-[1.5] text-faint [text-wrap:pretty]">
        Sign-ins aren&apos;t listed here. Look up sign-ins reads them from Zitadel&apos;s own log.
      </p>
      <Button variant="outline" size="sm" onClick={() => setLookingUp(true)}>
        Look up sign-ins
      </Button>
    </div>
    {lookingUp && <SignInLookup userId={userId} name={name} onClose={() => setLookingUp(false)} />}
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
              {group.day}
            </div>
            {group.entries.map((entry) => (
              <ActivityRow key={entry.id} entry={entry} userId={userId} />
            ))}
          </div>
        ))}

        {!atCap && (
          <PaginationRow
            shown={rows.length}
            hasMore={read >= limit || entries.isPlaceholderData}
            since={rows.length ? formatShortDate(rows[rows.length - 1].created_at) : null}
            pending={entries.isPlaceholderData}
            onMore={paging.more}
            onBack={paging.back}
          />
        )}
        {atCap && (
          <div className="flex flex-wrap items-center justify-between gap-x-4 border-t border-line px-5 py-3 text-[13px] text-faint">
            <span>Showing the most recent 200 entries, not their whole history.</span>
            <ArrowLink href={`/audit?user=${encodeURIComponent(userId)}`}>Audit</ArrowLink>
          </div>
        )}
      </ListStates>
    </Card>

    <OnTheTargets userId={userId} name={name} />
    </div>
  );
}

/**
 * Groups already-sorted entries by their local calendar day, labelled with
 * the same `formatShortDate` string used as the key — so the heading and the
 * grouping boundary can never disagree with each other, or with the row's clock.
 */
function groupByLocalDay(rows: AuditEntry[]): Array<{ day: string; entries: AuditEntry[] }> {
  const groups: Array<{ day: string; entries: AuditEntry[] }> = [];
  for (const entry of rows) {
    const day = formatShortDate(entry.created_at);
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.entries.push(entry);
    else groups.push({ day, entries: [entry] });
  }
  return groups;
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
        <TargetActivityCard key={entry.target} target={entry.target} kind={entry.kind} userId={userId} name={name} />
      ))}
    </>
  );
}

function TargetActivityCard({
  target,
  kind,
  userId,
  name,
}: {
  target: string;
  kind?: string;
  userId: string;
  name: string;
}) {
  const nameOf = useSystemNames();
  const activity = useTargetActivity(target, userId);
  const again = useTargetActivityAgain(target, userId);
  const data = activity.data;
  const label = nameOf(target);

  // Nothing at all rather than an empty card: a person with no account on a
  // target has no activity to be missing, and a card saying so on every target
  // in the deployment is noise on the common case.
  if (activity.isError) return null;

  const events = data?.readable ? (data.events ?? []) : [];
  const uncovered = sortNames(data?.uncovered ?? []);
  // Shares are a storage word: the storage kind says them, any other kind says "places".
  const storage = isStorage({ kind });
  // How old the log is: its own read, or the cooldown it ran into with nothing kept to show.
  const age: ReadState | null =
    data?.readable && data.taken_at
      ? observed({ taken_at: data.taken_at, current: data.current ?? true, stale: data.stale })
      : data?.reason === "cooldown" && data.retry_after
        ? { stale: { reason: STALE.cooldown, retry_after: data.retry_after } }
        : null;

  return (
    <Card>
      <div className="row-divider flex flex-wrap items-baseline gap-2 px-5 py-3">
        <span className="text-[14.5px] font-semibold">On {label}</span>
        <span className="text-[13px] text-muted">
          Read from {label}&rsquo;s own audit log, not from Syndra&rsquo;s record.
        </span>
      </div>

      <ListStates
        isLoading={activity.isLoading}
        error={null}
        errorTitle={`Couldn't read ${label}'s audit log.`}
        isEmpty={data?.readable === true && events.length === 0}
        skeleton={<RowSkeleton rows={3} avatar={false} label={`Reading ${label}`} />}
        empty={
          <EmptyState
            title={`No recorded activity for ${name} on ${label}.`}
            guidance={
              uncovered.length > 0
                ? storage
                  ? STORAGE_GUIDANCE
                  : UNCOVERED_GUIDANCE
                : `${label} has no activity for this account.`
            }
          />
        }
      >
        <ActivityRows events={events} />
      </ListStates>

      {/* The distinction the whole card exists for. "Could not look" and
          "nothing happened" are the same empty list otherwise, and they are
          opposite answers. `no_account` is a THIRD thing again: not a failed
          read, just nothing there to read — neutral tone, no warning colour,
          and never a raw status code (the backend already keeps those out of
          `detail`). */}
      {age && (
        <div className="border-t border-line px-5 py-3">
          <ReadFreshness
            state={age}
            subject={label}
            reader={{ audience: "operator", person: name }}
            onRefresh={again.press}
          />
        </div>
      )}
      {data && !data.readable && data.reason === "no_account" ? (
        <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
          {label} has no account for this person, so there is nothing to read
          there.
        </div>
      ) : data && !data.readable && !age ? (
        <div className="border-t border-line p-5">
          <Notice tone="warn" title={`${label}\u2019s audit log could not be read.`} next="Reload the page in a minute. If it keeps failing, ask whoever runs the Syndra server.">
            This is not a claim that nothing happened.{data.detail ? ` ${data.detail}` : ""}
          </Notice>
        </div>
      ) : null}

      {uncovered.length > 0 && (
        <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
          {storage ? (
            <UncoveredNote shares={uncovered} name={name} />
          ) : (
            uncoveredNote(uncovered, { audience: "operator", person: name })
          )}
        </div>
      )}
    </Card>
  );
}

function ActivityRow({ entry, userId }: { entry: AuditEntry; userId: string }) {
  const { verb, passive, destructive } = describeAction(entry.action, useSystemNames());
  const direction = actedOn(entry, userId);

  return (
    <div className="row-divider flex flex-wrap items-baseline gap-4 px-5 py-3">
      <span className="w-[64px] shrink-0 text-[12.5px] text-faint">
        {formatClock(entry.created_at)}
      </span>

      {/* The sentence changes with direction, because "Gave a role"
          beside somebody's name means two opposite things depending on whether
          they were the hand or the recipient.

          Acted: the SHARED renderer, which knows a target_id is a person on
          most actions and a bundle on a bundle edit. This row used to print a
          bare verb and hand every target to the person resolver, so it read
          "Gave somebody a bundle" — on the page of the somebody — and would
          have called a bundle an unknown account.

          Affected: the passive, because the person it happened to is the
          heading of this page. "Gave Bobby Roode a bundle" on
          Bobby's own Activity tab says his name twice and buries the one
          fact the row adds, which is who did it. */}
      <span className="min-w-[240px] flex-1 text-[14px]">
        {isPasswordSend(entry) ? (
          <SendRow entry={entry} />
        ) : direction === "acted" ? (
          <EventSentence entry={entry} />
        ) : (
          <>
            <span className={destructive ? "font-semibold text-danger-text" : "font-semibold"}>
              {passive ?? verb}
            </span>
            {direction === "both" ? (
              <span className="text-muted">, by themselves</span>
            ) : (
              <span className="text-muted">
                {", by "}
                <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
              </span>
            )}
          </>
        )}
      </span>

      <TraceCell entry={entry} className="w-[86px] shrink-0 text-right" />
    </div>
  );
}

/** A password change on this person's record: what happened, who did it, then its sends, faint (A-13 §4). */
function SendRow({ entry }: { entry: AuditEntry }) {
  const { headline, sends } = describeSends(entry, useSystemNames());
  return (
    <span className="flex flex-col gap-0.5">
      <span>
        <span className="font-semibold">{headline}</span>
        <span className="text-muted">
          {", by "}
          <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
        </span>
      </span>
      {sends && <span className="text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">{sends}</span>}
    </span>
  );
}

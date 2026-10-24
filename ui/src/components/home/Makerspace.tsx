"use client";

import Link from "next/link";
import { useMemo } from "react";

import { UserName } from "@/components/names";
import { RowSkeleton } from "@/components/states";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { StatCard } from "@/components/ui/StatCard";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { machineName } from "@/lib/audit-vocabulary";
import { EventSentence } from "@/components/audit/EventLine";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { formatShortDate, formatWhen, humanizeKey } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { hasAccess, isDeparted, isPerson, peopleHref } from "@/lib/people-filters";
import { useAuditEntries } from "@/lib/queries/useAudit";
import { useBundles } from "@/lib/queries/useBundles";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useTargets } from "@/lib/queries/useTargets";
import { useUsers } from "@/lib/queries/useUsers";

/**
 * The makerspace, below the work.
 *
 * Home's original contract was "actionable work only — no counts you cannot
 * act on, no charts", and it was right about the top of the page: the queue
 * must never be pushed down by anything. But the contract assumed the queue is
 * always non-empty, and most days it isn't. An operator who lands on "Nothing
 * needs you." and a blank page learns nothing about the space they run, so they
 * go looking — which is the navigation the landing page existed to prevent.
 *
 * So the work stays on top, always, and this sits underneath it, always. The
 * rule that survives is the useful half: **every number here is a link into the
 * thing it counts.** No charts, no trends, nothing you can only look at.
 */
export function Makerspace() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-baseline gap-3 pt-2">
        <h2 className="type-section-title">The makerspace</h2>
        <span className="text-[13.5px] text-faint">
          Not your queue — the shape of the place. Every number opens what it counts.
        </span>
      </div>

      <Tiles />
      <div className="grid gap-6 desktop:grid-cols-2">
        <Gaps />
        <AccessShape />
      </div>
      <RecentActivity />
    </div>
  );
}

/**
 * Three tiles, each the link to the list it counts (route-home; StatCard `href`).
 * They used to be four machine-health cells, and three of those counted what the
 * queue above already shows as a block — pending changes, drift, expiring access —
 * so a number was given twice and neither copy said which was the one to act on.
 * What the queue cannot say is how the place stands: whether every system was
 * read, how many people there are, and what access is made of.
 *
 * No count is ever shown before it has been read: a tile whose list is still
 * loading says so rather than reporting a zero.
 */
function Tiles() {
  const summary = useGovernanceSummary();
  const targets = useTargets();
  const users = useUsers("");
  const roles = useGlobalRoleCatalog();
  const bundles = useBundles();

  // A count that was never read is not zero, and a read that failed is not "all clear":
  // each tile says it hasn't read yet, or couldn't just now, instead of claiming either.
  const systemsPending = targets.isLoading || summary.isLoading;
  const systemsFailed = !!(targets.error || summary.error);
  const systemsKnown = !systemsPending && !systemsFailed;
  const endingKnown = !summary.isLoading && !summary.error;
  const notRead = (loadingNow: boolean, failed: boolean) =>
    loadingNow ? "Reading now." : failed ? "Not read just now." : "";

  const registered = (targets.data ?? []).filter((entry) => entry.registered !== false);
  const unread = summary.data?.unreconciled_targets ?? [];
  const read = registered.length - unread.length;

  const endingSoon = new Set((summary.data?.expiring_grants ?? []).map((grant) => grant.user_id)).size;

  const roleRows = roles.data ?? [];
  const projects = new Set(roleRows.map((role) => role.project_id)).size;
  const bundleCount = (bundles.data ?? []).length;
  const rolesLoading = roles.isLoading || bundles.isLoading;
  const rolesFailed = !!(roles.error || bundles.error);
  const rolesKnown = !rolesLoading && !rolesFailed;

  return (
    <div className="flex flex-wrap gap-4">
      <StatCard
        label="Connected systems"
        href="/system/targets"
        value={!systemsKnown ? "—" : registered.length === 0 ? "None" : `${read} of ${registered.length} read`}
        tone={systemsKnown && unread.length > 0 ? "warn" : "neutral"}
        detail={
          !systemsKnown
            ? notRead(systemsPending, systemsFailed)
            : registered.length === 0
            ? "No system is connected."
            : unread.length === 0
              ? "Every system answered its last read."
              : unread.length === 1
                ? `${targetLabel(unread[0].target)} has not been read since ${formatShortDate(unread[0].since)}.`
                : `${unread.length} systems have not been read.`
        }
      />
      <StatCard
        label="People"
        href="/users"
        value={users.isLoading || users.error || !users.data ? "—" : String(users.data.filter(isPerson).length)}
        detail={
          !endingKnown
            ? notRead(!!summary.isLoading, !!summary.error)
            : endingSoon === 0
              ? "Nobody's access ends in the next 14 days."
              : `${endingSoon} whose access ends in 14 days`
        }
      />
      <StatCard
        label="Where access lives"
        href="/projects"
        value={!rolesKnown ? "—" : `${roleRows.length} ${roleRows.length === 1 ? "role" : "roles"}`}
        detail={
          !rolesKnown
            ? rolesLoading
              ? "Reading the catalogue."
              : "Not read just now."
            : `Across ${projects} ${projects === 1 ? "project" : "projects"} and ${bundleCount} ${bundleCount === 1 ? "bundle" : "bundles"}`
        }
      />
    </div>
  );
}

/**
 * The two ends of a person's life here that nothing else watches: arrived and
 * never set up, left and never cleaned up. Both are genuinely actionable, both
 * open a People view already narrowed to exactly those people — and from there,
 * bulk mode is one click away.
 */
function Gaps() {
  const users = useUsers("");
  const rows = useMemo(() => users.data ?? [], [users.data]);

  // Neither "still reading" nor "couldn't read" is an answer about anybody.
  if (users.isLoading || users.error || !users.data) {
    return (
      <Card>
        <CardHeader title="Gaps" />
        {users.isLoading ? (
          <RowSkeleton rows={3} label="Loading gaps" />
        ) : (
          <div className="px-5 py-3.5 text-[14px] text-faint">Not read just now.</div>
        )}
      </Card>
    );
  }

  // A service account with no roles is not somebody who arrived and was never
  // set up; one that left and still holds roles is exactly what the next row is for.
  const noAccess = rows.filter((entry) => isPerson(entry) && !hasAccess(entry)).length;
  const departedHolding = rows.filter(
    (entry) => isDeparted(entry.user.status) && hasAccess(entry),
  ).length;

  return (
    <Card>
      <CardHeader title="Gaps" />
      <GapRow
        count={noAccess}
        href={peopleHref({ attention: "no-access" })}
        headline={noAccess === 1 ? "person has no access at all" : "people have no access at all"}
        note="Arrived, never set up."
        clear="Everybody here has something."
      />
      <GapRow
        count={departedHolding}
        href={peopleHref({ attention: "departed" })}
        headline={departedHolding === 1 ? "departed account still holds roles" : "departed accounts still hold roles"}
        note="Left, never cleaned up."
        clear="Nobody who left still holds anything."
      />
    </Card>
  );
}

function GapRow({
  count,
  href,
  headline,
  note,
  clear,
}: {
  count: number;
  href: string;
  headline: string;
  note: string;
  clear: string;
}) {
  if (count === 0) {
    return (
      <div className="row-divider flex items-baseline gap-3 px-5 py-3.5">
        <span className="text-[14px] text-faint">{clear}</span>
      </div>
    );
  }

  return (
    <Link
      href={href}
      className="row-divider flex items-baseline gap-3 px-5 py-3.5 hover-tint"
    >
      <span className="font-display text-[20px] leading-none text-accent-text">{count}</span>
      <span className="text-[14.5px]">{headline}</span>
      <span className="flex-1" />
      <span className="text-[13px] text-faint">{note}</span>
    </Link>
  );
}

/**
 * Where access actually lives. Not a chart — a short list of the roles most
 * people hold, and the count of catalogue entries nobody holds at all, each
 * one a link to the people or the role behind it.
 */
function AccessShape() {
  const roles = useGlobalRoleCatalog();
  const bundles = useBundles();

  const top = useMemo(
    () =>
      [...(roles.data ?? [])]
        .filter((role) => (role.observed_user_count ?? role.assigned_user_count) > 0)
        .sort(
          (a, b) =>
            (b.observed_user_count ?? b.assigned_user_count) -
            (a.observed_user_count ?? a.assigned_user_count),
        )
        .slice(0, 5),
    [roles.data],
  );
  const unused = (roles.data ?? []).filter((role) => role.is_unused).length;
  const emptyBundles = (bundles.data ?? []).filter((bundle) => !bundle.holder_count).length;

  // One basis for the whole tile — every row carries an identical copy, so
  // the freshness is stated here once rather than once per row.
  const basis = roles.data?.[0]?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  const loading = roles.isLoading || bundles.isLoading;
  const failed = !loading && !!(roles.error || bundles.error);

  return (
    <Card>
      <CardHeader title="Roles most people hold" />
      {loading ? (
        <RowSkeleton rows={3} label="Loading where access lives" />
      ) : failed ? (
        <div className="px-5 py-3.5 text-[14px] text-faint">Not read just now.</div>
      ) : (
        <>
          {top.length > 0 && (
            <div className="px-5 pb-3">
              <ReadFreshness state={readState} subject="What Zitadel confirmed" />
            </div>
          )}
          {top.length === 0 ? (
            <div className="px-5 py-3.5 text-[14px] text-faint">Nobody holds a role yet.</div>
          ) : (
            top.map((role) => {
              const holders = holdersLine(
                role.assigned_user_count,
                role.confirmed_user_count,
                role.observed_user_count,
              );
              return (
                <Link
                  key={`${role.project_id}:${role.role_key}`}
                  // The role's own page: its number is what Zitadel holds,
                  // and People lists only those Syndra has a record of, so
                  // the unexplained holders the note counts never appeared.
                  href={`/projects/${encodeURIComponent(role.project_id)}/roles/${encodeURIComponent(role.role_key)}`}
                  className="row-divider flex items-baseline gap-3 px-5 py-3 hover-tint"
                >
                  <span className="w-[36px] shrink-0 font-display text-[18px] leading-none">
                    {holders.headline}
                  </span>
                  {/* The project is the trailing column, so the name slot carries the
                      role alone — the pair is established by the row, the same way
                      the roles index establishes it with a Project column. */}
                  <span className="min-w-0 flex-1 truncate text-[14.5px]">
                    {role.display_name || humanizeKey(role.role_key)}
                  </span>
                  <span className="shrink-0 truncate text-right text-[13px] text-faint">
                    {role.project_name}
                    {/* What Zitadel shows right now, not Syndra's record of who
                        was granted this — see the confirmation line above. */}
                    {holders.note && (
                      <span className={`block text-[12.5px] ${holdersToneClass[holders.tone]}`}>
                        {holders.note}
                      </span>
                    )}
                  </span>
                </Link>
              );
            })
          )}
        </>
      )}

      {/* Dead catalogue entries are not urgent, so they read as one quiet line
          rather than a block of their own.

          The two links carry their own 44px on touch. WCAG 2.5.8 exempts an
          inline target inside a sentence, and these are not in one: the row
          holds two destinations and a separator and no prose, so the exemption
          does not reach them. They measured 16px tall on the screen this
          product opens on. */}
      {!loading && !failed && (
        <div className="flex flex-wrap items-center gap-x-1.5 border-t border-line px-5 py-3 text-[13px] text-faint desktop:block desktop:py-3">
          {unused === 0 && emptyBundles === 0 ? (
            "Every role and bundle is in use."
          ) : (
            <>
              {unused > 0 && (
                <Link
                  href="/roles?unused=1"
                  className="inline-flex min-h-11 items-center font-semibold text-accent-text desktop:min-h-6 motion-tint hover:text-ink"
                >
                  {unused} {unused === 1 ? "role" : "roles"} nobody holds
                </Link>
              )}
              {unused > 0 && emptyBundles > 0 && <span aria-hidden>·</span>}
              {emptyBundles > 0 && (
                <Link
                  href="/bundles"
                  className="inline-flex min-h-11 items-center font-semibold text-accent-text desktop:min-h-6 motion-tint hover:text-ink"
                >
                  {emptyBundles} empty {emptyBundles === 1 ? "bundle" : "bundles"}
                </Link>
              )}
            </>
          )}
        </div>
      )}
    </Card>
  );
}

/**
 * Proof the place is alive on a day when nothing needs you. Eight lines, names
 * resolved, and a way into the full log.
 */
function RecentActivity() {
  const entries = useAuditEntries({ limit: 8 });
  const rows = entries.data ?? [];
  // Nothing is "recorded" until the log has been read.
  const empty = entries.isLoading ? (
    <RowSkeleton rows={3} label="Loading recent activity" />
  ) : entries.error ? (
    <div className="px-5 py-3.5 text-[14px] text-faint">Not read just now.</div>
  ) : (
    <div className="px-5 py-3.5 text-[14px] text-faint">Nothing recorded yet.</div>
  );

  return (
    <Card>
      <CardHeader
        title="Lately"
        action={
          <CardHeaderLink href="/audit">Full audit log →</CardHeaderLink>
        }
      />
      {rows.length === 0 ? (
        empty
      ) : (
        rows.map((entry) => (
          <div key={entry.id} className="row-divider flex flex-wrap items-baseline gap-3 px-5 py-2.5">
            {/* formatWhen prints a bare time for today ("14:32") but a full
                date for anything older ("10 Sept 14:32") — wider than the
                clock-only column this used to be. */}
            <span className="w-[92px] shrink-0 text-[12.5px] text-faint">
              {formatWhen(entry.created_at)}
            </span>
            <span className="w-[150px] shrink-0 truncate text-[14px] font-semibold">
              <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
            </span>
            {/* The SAME renderer the Audit page uses. This panel used to hand
                every target_id to the person resolver, so a bundle edit read
                "Unknown account 9e93893d-…" here and "Community · Basic" there
                — one event, two screens, disagreeing. */}
            <span className="min-w-[min(240px,100%)] flex-1 text-[14px] text-muted">
              <EventSentence entry={entry} />
            </span>
          </div>
        ))
      )}
    </Card>
  );
}

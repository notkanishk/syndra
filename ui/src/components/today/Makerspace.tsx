"use client";

import Link from "next/link";
import { useMemo } from "react";

import { UserName } from "@/components/names";
import { Card, CardHeader } from "@/components/ui/Card";
import { describeAction, machineName } from "@/lib/audit-vocabulary";
import { formatClock } from "@/lib/format";
import { peopleHref } from "@/lib/people-filters";
import { useAuditEntries } from "@/lib/queries/useAudit";
import { useBundles } from "@/lib/queries/useBundles";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useUsers } from "@/lib/queries/useUsers";
import { isDeparted } from "@/lib/people-filters";

/**
 * The makerspace, below the work.
 *
 * Today's original contract was "actionable work only — no counts you cannot
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

      <Health />
      <div className="grid gap-6 lg:grid-cols-2">
        <Gaps />
        <AccessShape />
      </div>
      <RecentActivity />
    </div>
  );
}

/**
 * Is the machine healthy? Four facts, each a link to the surface that owns it.
 * A cell that is fine says so quietly; a cell that isn't takes its semantic
 * colour and stops being quiet.
 */
function Health() {
  const summary = useGovernanceSummary();
  const propagation = summary.data?.pending_propagation;
  const drift = summary.data?.drift;

  const reachable = propagation?.zitadel_reachable ?? true;
  const queued = propagation?.count ?? 0;
  const unexplained = drift?.count ?? 0;

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <HealthCell
        label="Identity provider"
        value={reachable ? "Reachable" : "Unreachable"}
        tone={reachable ? "calm" : "danger"}
        href="/system"
        note={reachable ? "Writes are landing" : "Writes stay queued — nothing is lost"}
      />
      <HealthCell
        label="Queued writes"
        value={String(queued)}
        tone={queued > 0 ? "accent" : "calm"}
        href="/operations"
        note={queued > 0 ? "Waiting for a drain" : "Nothing buffered"}
      />
      <HealthCell
        label="Unexplained access"
        value={String(unexplained)}
        tone={unexplained > 0 ? "danger" : "calm"}
        href="/governance/drift"
        note={unexplained > 0 ? "Out-of-band changes to triage" : "Everything is accounted for"}
      />
      <HealthCell
        label="Expiring inside the window"
        value={String(summary.data?.expiring_grants.length ?? 0)}
        tone={(summary.data?.expiring_grants.length ?? 0) > 0 ? "warn" : "calm"}
        href="/review/expiring-access"
        note="Direct grants with a deadline"
      />
    </div>
  );
}

const TONE_CLASS = {
  calm: "text-ink",
  accent: "text-accent-text",
  warn: "text-warn-text",
  danger: "text-danger-text",
} as const;

function HealthCell({
  label,
  value,
  note,
  href,
  tone,
}: {
  label: string;
  value: string;
  note: string;
  href: string;
  tone: keyof typeof TONE_CLASS;
}) {
  return (
    <Link
      href={href}
      className="panel flex flex-col gap-1 px-4 py-3.5 transition-colors hover:bg-[var(--hover)]"
    >
      <span className="type-label">{label}</span>
      <span className={`font-display text-[26px] leading-none ${TONE_CLASS[tone]}`}>{value}</span>
      <span className="text-[12.5px] text-faint">{note}</span>
    </Link>
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

  const noAccess = rows.filter((entry) => entry.effective_role_count === 0).length;
  const departedHolding = rows.filter(
    (entry) => isDeparted(entry.user.status) && entry.effective_role_count > 0,
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
      className="row-divider flex items-baseline gap-3 px-5 py-3.5 transition-colors hover:bg-[var(--hover)]"
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
        .filter((role) => role.assigned_user_count > 0)
        .sort((a, b) => b.assigned_user_count - a.assigned_user_count)
        .slice(0, 5),
    [roles.data],
  );
  const unused = (roles.data ?? []).filter((role) => role.is_unused).length;
  const emptyBundles = (bundles.data ?? []).filter((bundle) => !bundle.holder_count).length;

  return (
    <Card>
      <CardHeader title="Where access lives" />
      {top.length === 0 ? (
        <div className="px-5 py-3.5 text-[14px] text-faint">Nobody holds a role yet.</div>
      ) : (
        top.map((role) => (
          <Link
            key={`${role.project_id}:${role.role_key}`}
            href={peopleHref({ project: role.project_id, role: role.role_key })}
            className="row-divider flex items-baseline gap-3 px-5 py-3 transition-colors hover:bg-[var(--hover)]"
          >
            <span className="w-[36px] shrink-0 font-display text-[18px] leading-none">
              {role.assigned_user_count}
            </span>
            <span className="min-w-0 flex-1 truncate text-[14.5px]">
              {role.display_label || role.role_key}
            </span>
            <span className="shrink-0 truncate text-[13px] text-faint">{role.project_name}</span>
          </Link>
        ))
      )}

      {/* Dead catalogue entries are not urgent, so they read as one quiet line
          rather than a block of their own. */}
      <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
        {unused === 0 && emptyBundles === 0 ? (
          "Every role and bundle is in use."
        ) : (
          <>
            {unused > 0 && (
              <Link href="/roles" className="font-semibold text-accent-text">
                {unused} {unused === 1 ? "role" : "roles"} nobody holds
              </Link>
            )}
            {unused > 0 && emptyBundles > 0 && " · "}
            {emptyBundles > 0 && (
              <Link href="/bundles" className="font-semibold text-accent-text">
                {emptyBundles} empty {emptyBundles === 1 ? "bundle" : "bundles"}
              </Link>
            )}
          </>
        )}
      </div>
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

  return (
    <Card>
      <CardHeader
        title="Lately"
        action={
          <Link href="/audit" className="text-[13.5px] font-semibold text-accent-text">
            Full audit log →
          </Link>
        }
      />
      {rows.length === 0 ? (
        <div className="px-5 py-3.5 text-[14px] text-faint">Nothing recorded yet.</div>
      ) : (
        rows.map((entry) => {
          const { verb, destructive } = describeAction(entry.action);
          return (
            <div key={entry.id} className="row-divider flex flex-wrap items-baseline gap-3 px-5 py-2.5">
              <span className="w-[46px] shrink-0 text-[12.5px] text-faint">
                {formatClock(entry.created_at)}
              </span>
              <span className="w-[150px] shrink-0 truncate text-[14px] font-semibold">
                <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
              </span>
              <span className="min-w-0 flex-1 truncate text-[14px] text-muted">
                <span className={destructive ? "font-semibold text-danger-text" : undefined}>
                  {verb}
                </span>
                {entry.target_id && entry.target_id !== "-" && entry.target_id !== "system" ? (
                  <>
                    {" — "}
                    <UserName id={entry.target_id} />
                  </>
                ) : null}
              </span>
            </div>
          );
        })
      )}
    </Card>
  );
}

"use client";

import { Term } from "@/components/ui/Term";
import { useSearchParams } from "next/navigation";
import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { BundleName, RoleRef, UserName } from "@/components/names";
import { humanizeKey } from "@/lib/format";
import { useMappingRules, type MappingRuleRow } from "@/lib/queries/useMappingRules";
import { useCascadeGroups, type CascadeGroupRow } from "@/lib/queries/useConfirmationMode";
import { Relative } from "@/components/ui/Time";

/**
 * S4 · Automation › Change history.
 *
 * One entry per cascade, newest first — not one row per change. Each entry is a
 * sentence about consequence rather than a diff, and "8 sent", "2 waiting",
 * "no changes" is the whole vocabulary. Cascades whose changes have not landed
 * are shown too: a half-applied cascade is the thing that creates unexplained
 * access, and hiding it until it settles is how it goes unnoticed.
 */
export default function ChangeHistoryPage() {
  // Where an audit row's trace link lands. One cascade, named — the audit log reaches back
  // further than this page's fifty, so the narrowing is done by the query, not by searching the
  // glance list for a row that may not be in it.
  const asked = useSearchParams().get("cascade")?.trim() ?? "";
  const cascades = useCascadeGroups(asked || undefined);
  const rows = cascades.data ?? [];
  const rules = useMappingRules().data;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Change history"
        lede={
          asked
            ? "One edit, followed from the audit entry that recorded it, and what it changed for people."
            : (
                <>
                  What each edit to a <Term name="bundle">bundle</Term> or an automatic rule then
                  changed for people in Zitadel, newest first, one{" "}
                  <Term name="cascade">cascade</Term> per row. Audit records who did what; this
                  page records what those edits set off.
                </>
              )
        }
        actions={asked ? <ArrowLink href="/operations/cascades">Show every edit</ArrowLink> : undefined}
      />

      {/* This list is bundle/rule/lifecycle cascades only — a single-user direct
          grant never appears here even though it shows up in the audit feed,
          so a short window with mostly direct grants renders as one lonely
          row here. Without this line that reads as "almost nothing happened"
          rather than "most of the period wasn't a cascade at all". */}
      {!asked && (
        <p className="text-[13px] text-faint">
          Cascades only: a bundle edit, an automatic rule, or someone&rsquo;s status changing.
          Direct, one-person grants don&rsquo;t appear here even though they&rsquo;re in the audit
          feed. Shows up to the 50 most recent.
        </p>
      )}

      <ListStates
        isLoading={cascades.isLoading}
        error={cascades.error}
        isEmpty={rows.length === 0}
        onRetry={() => cascades.refetch()}
        errorTitle="Couldn't load change history."
        skeleton={
          <Card>
            <RowSkeleton rows={4} label="Loading change history" />
          </Card>
        }
        empty={
          <Card>
            {asked ? (
              // Not "no edits yet" — something did happen, or there would be no audit row
              // pointing here. The changes it produced are no longer in the queue, and saying that
              // plainly is the difference between a page that looks broken and one that answers.
              <EmptyState
                title="That edit has finished."
                guidance="Every change it produced has gone through and been cleared. The audit entry that brought you here is still the record of what happened."
              />
            ) : (
              <EmptyState
                title="No edits yet."
                guidance="When a bundle or an automatic rule is edited, what it changed for people appears here."
              />
            )}
          </Card>
        }
      >
        <div className="flex flex-col gap-4">
          {rows.map((group) => (
            <CascadeCard key={group.cascade_id} group={group} rules={rules} one={Boolean(asked)} />
          ))}
        </div>
      </ListStates>
    </div>
  );
}

/** `one`: this is the single-edit view, so every write is listed; otherwise the rest are one click away. */
function CascadeCard({ group, rules, one }: { group: CascadeGroupRow; rules?: MappingRuleRow[]; one: boolean }) {
  const first = group.writes[0];
  // The rule by what it does: it has no name of its own. A bundle by its name.
  const rule = group.source === "rule" ? rules?.find((entry) => entry.id === first?.source_ref) : undefined;

  return (
    <Card>
      <CardHeader
        title={<span className="type-card-title">{titleFor(group)}</span>}
        note={<Relative iso={group.settled_at ?? group.started_at} />}
        action={<StatePill group={group} />}
      />

      <div className="row-divider flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3.5">
        <p className="text-[14px] leading-[1.55] text-muted">{consequenceFor(group)}</p>
        {group.waiting + group.failed > 0 && <ArrowLink href="/governance/pending">Pending changes</ArrowLink>}
      </div>

      {group.reason && <p className="row-divider px-5 py-3 text-[14px] leading-[1.55] text-ink">“{group.reason}”</p>}

      <div className="row-divider flex flex-wrap gap-x-5 gap-y-1.5 px-5 py-3 text-[13.5px]">
        {(one ? group.writes : group.writes.slice(0, 6)).map((write) => (
          <span key={write.id} className="text-muted">
            <UserName id={write.user_id} /> ·{" "}
            {(write.role_keys ?? []).map((key) => (
              <RoleRef key={key} projectId={write.project_id} roleKey={key} className="mr-2" />
            ))}
          </span>
        ))}
        {!one && group.writes.length > 6 && (
          <Link
            href={`/operations/cascades?cascade=${encodeURIComponent(group.cascade_id)}`}
            className="font-semibold text-accent-text link-hover"
          >
            and {group.writes.length - 6} more
          </Link>
        )}
        {group.source === "bundle" && first?.source_ref && (
          <span className="text-faint">
            from <BundleName id={first.source_ref} fallback="a bundle that is gone" />
          </span>
        )}
        {rule && (
          <span className="text-faint">
            from the rule {humanizeKey(rule.source_role)} gives {humanizeKey(rule.target_role)}
          </span>
        )}
      </div>
    </Card>
  );
}

/**
 * Warn when changes are still waiting, healthy when everything went through
 * AND Zitadel has confirmed it, neutral when an edit produced no changes at
 * all, which is a real and reassuring outcome, not an empty state.
 *
 * "Applied" is Syndra's own record of having sent the write; "confirmed" is a
 * post-write read of Zitadel agreeing it landed. Read-back can lag sending by
 * a cycle, so a just-applied cascade legitimately shows sent-but-not-yet-
 * confirmed for a little while. That is a fact worth a word of its own, not
 * something to fold into "sent".
 */
function StatePill({ group }: { group: CascadeGroupRow }) {
  if (group.failed > 0) return <StatusSeat tone="danger" word={`${group.failed} failed`} />;
  if (group.waiting > 0) return <StatusSeat tone="warn" word={`${group.waiting} waiting for you to send`} />;
  if (group.applied > 0) {
    return group.confirmed === group.applied ? (
      <StatusSeat tone="healthy" word={`${group.applied} confirmed by Zitadel`} />
    ) : (
      <StatusSeat tone="warn" word={`${group.applied} sent, ${group.confirmed} confirmed`} />
    );
  }
  return <StatusSeat tone="neutral" word="no changes" />;
}

function titleFor(group: CascadeGroupRow): string {
  const people =
    group.user_ids.length === 1
      ? "one person"
      : `${group.user_ids.length} people`;
  if (group.source === "rule") return `An automatic rule applied to ${people}`;
  if (group.source === "bundle") return `A bundle edit reached ${people}`;
  return `Somebody's status changed, affecting ${people}`;
}

function consequenceFor(group: CascadeGroupRow): string {
  const total = group.applied + group.waiting + group.failed;
  const needed = total === 1 ? "1 change was needed" : `${total} changes were needed`;

  if (group.failed > 0) {
    return `${needed}. ${group.applied} went through and ${group.failed} failed on the way to Zitadel. The failed ones are still waiting for you to send.`;
  }
  if (group.waiting > 0 && group.applied > 0) {
    return `${needed}. ${group.applied} went through and ${group.waiting} are still waiting for you to send. Until they all go through, some people have access that does not match the edit, so send them.`;
  }
  if (group.waiting > 0) {
    return `${needed}, and none have reached Zitadel yet. Nothing has changed for anybody so far.`;
  }
  if (group.confirmed === group.applied) {
    return `${needed}, and all of them went through. Nothing further is waiting from this edit.`;
  }
  const unconfirmed = group.applied - group.confirmed;
  return `${needed}. Sent; Zitadel has not yet confirmed ${unconfirmed}.`;
}

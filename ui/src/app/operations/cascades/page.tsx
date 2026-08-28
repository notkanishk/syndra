"use client";

import { Term } from "@/components/ui/Term";
import Link from "next/link";
import { useSearchParams } from "next/navigation";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { shortId } from "@/lib/audit-vocabulary";
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

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Change history"
        lede={
          asked
            ? "One edit, followed from the audit entry that recorded it, and what it changed for people."
            : (
                <>
                  What each edit to a <Term name="bundle">bundle</Term> or an automatic rule then
                  changed for people in Zitadel, newest first — one{" "}
                  <Term name="cascade">cascade</Term> per row. Audit records who did what; this
                  page records what those edits set off.
                </>
              )
        }
        actions={
          asked ? (
            <Link href="/operations/cascades" className="text-[13.5px] font-semibold text-accent-text">
              Show every edit
            </Link>
          ) : undefined
        }
      />

      <p className="text-[13px] text-faint">
        Handles: c_ is one edit&rsquo;s set of changes, R_ an automatic rule, b_ a bundle.
      </p>

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
        <div className="flex flex-col gap-[18px]">
          {rows.map((group, index) => (
            <CascadeCard key={group.cascade_id} group={group} newest={!asked && index === 0} />
          ))}
        </div>
      </ListStates>
    </div>
  );
}

function CascadeCard({ group, newest }: { group: CascadeGroupRow; newest: boolean }) {
  const first = group.writes[0];

  return (
    <Card>
      <CardHeader
        title={
          <span className="flex flex-wrap items-baseline gap-2.5">
            <Mono className={newest ? "text-[13px] text-accent-text" : "text-[13px] text-faint"}>
              {shortId(group.cascade_id, "c")}
            </Mono>
            <span className="type-card-title">{titleFor(group)}</span>
          </span>
        }
        note={<Relative iso={group.settled_at ?? group.started_at} />}
        action={<StatePill group={group} />}
      />

      <p className="row-divider px-5 py-3.5 text-[14px] leading-[1.55] text-muted">
        {consequenceFor(group)}
      </p>

      <div className="row-divider flex flex-wrap gap-x-5 gap-y-1.5 px-5 py-3 text-[13.5px]">
        {group.writes.slice(0, 6).map((write) => (
          <span key={write.id} className="text-muted">
            <UserName id={write.user_id} /> · <ProjectName id={write.project_id} /> /{" "}
            {(write.role_keys ?? []).map((key) => (
              <Mono key={key} className="mr-1">
                {key}
              </Mono>
            ))}
          </span>
        ))}
        {group.writes.length > 6 && (
          <span className="text-faint">and {group.writes.length - 6} more</span>
        )}
        {first?.source_ref && (
          <span className="text-faint">
            from {group.source === "rule" ? "rule" : "bundle"}{" "}
            <Mono>{shortId(first.source_ref, group.source === "rule" ? "R" : "b")}</Mono>
          </span>
        )}
      </div>
    </Card>
  );
}

/**
 * Three words, and only three. Amber when changes are still waiting, accent
 * when everything went through, neutral when an edit produced no changes at
 * all — which is a real and reassuring outcome, not an empty state.
 */
function StatePill({ group }: { group: CascadeGroupRow }) {
  if (group.failed > 0) {
    return (
      <Badge tone="danger">
        {group.failed} failed
      </Badge>
    );
  }
  if (group.waiting > 0) {
    return <Badge tone="warn">{group.waiting} waiting to be sent</Badge>;
  }
  if (group.applied > 0) {
    return <Badge tone="accent">{group.applied} sent</Badge>;
  }
  return <Badge>no changes</Badge>;
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
    return `${needed}. ${group.applied} went through and ${group.failed} failed on the way to Zitadel. The failed ones are still waiting under Pending changes.`;
  }
  if (group.waiting > 0 && group.applied > 0) {
    return `${needed}. ${group.applied} went through and ${group.waiting} are still waiting to be sent. Until they all go through, some people have access that does not match the edit — send them from Pending changes.`;
  }
  if (group.waiting > 0) {
    return `${needed}, and none have reached Zitadel yet. Nothing has changed for anybody so far. Send them from Pending changes.`;
  }
  return `${needed}, and all of them went through. Nothing further is waiting from this edit.`;
}

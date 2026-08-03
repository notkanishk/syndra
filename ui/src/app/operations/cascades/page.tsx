"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { useCascadeGroups, type CascadeGroupRow } from "@/lib/queries/useConfirmationMode";
import { Relative } from "@/components/ui/Time";

/**
 * S4 · Automation › Change history.
 *
 * One entry per cascade, newest first — not one row per write. Each entry is a
 * sentence about consequence rather than a diff, and "8 applied", "2 waiting",
 * "no writes" is the whole vocabulary. Cascades whose writes have not landed
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
        meta={
          asked
            ? "One cascade, followed from the audit entry that caused it."
            : "What a bundle or rule change actually did downstream — one entry per cascade, newest first."
        }
        actions={
          asked ? (
            <Link href="/operations/cascades" className="text-[13.5px] font-semibold text-accent-text">
              Show all changes
            </Link>
          ) : undefined
        }
      />

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
              // Not "nothing has cascaded yet" — something did, or there would be no audit row
              // pointing here. The writes it produced are no longer in the queue, and saying that
              // plainly is the difference between a page that looks broken and one that answers.
              <EmptyState
                title="That cascade is no longer in the queue."
                guidance="The writes it produced have been carried out and cleared. The audit entry that brought you here is still the record of what happened."
              />
            ) : (
              <EmptyState
                title="Nothing has cascaded yet."
                guidance="Editing a bundle or a rule writes its downstream effect here."
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
            from <Mono>{shortId(first.source_ref, sourcePrefix(group.source))}</Mono>
          </span>
        )}
      </div>
    </Card>
  );
}

/**
 * Three words, and only three. Amber when writes are still waiting, accent when
 * everything landed, neutral when a cascade produced no writes at all — which
 * is a real and reassuring outcome, not an empty state.
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
    return (
      <Badge tone="warn">
        {group.waiting} {group.waiting === 1 ? "write" : "writes"} waiting
      </Badge>
    );
  }
  if (group.applied > 0) {
    return <Badge tone="accent">{group.applied} applied</Badge>;
  }
  return <Badge>no writes</Badge>;
}

function titleFor(group: CascadeGroupRow): string {
  const people =
    group.user_ids.length === 1
      ? "one person"
      : `${group.user_ids.length} people`;
  if (group.source === "rule") return `A rule fired for ${people}`;
  if (group.source === "bundle") return `A bundle change reached ${people}`;
  return `A lifecycle change reached ${people}`;
}

function consequenceFor(group: CascadeGroupRow): string {
  const total = group.applied + group.waiting + group.failed;
  const writes = `${total} ${total === 1 ? "write" : "writes"}`;

  if (group.failed > 0) {
    return `${writes} were produced; ${group.failed} failed on the way to the identity provider and ${group.applied} landed. The failed ones stay in the queue.`;
  }
  if (group.waiting > 0 && group.applied > 0) {
    return `${writes} were produced. ${group.applied} landed and ${group.waiting} are still queued — this cascade is half-applied, which is exactly the state that produces access nobody can explain later.`;
  }
  if (group.waiting > 0) {
    return `${writes} were produced and none have reached the identity provider yet. Nothing has changed for anybody so far.`;
  }
  return `${writes} were produced and all of them landed. Nothing further is pending from this change.`;
}

function sourcePrefix(source: string): string {
  return source === "rule" ? "R" : "b";
}

function shortId(id: string | undefined, prefix: string): string {
  if (!id) return "—";
  return `${prefix}_${id.replace(/-/g, "").slice(0, 4)}`;
}

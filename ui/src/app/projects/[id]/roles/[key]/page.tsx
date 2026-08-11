"use client";

import Link from "next/link";
import { use, useMemo, useState } from "react";

import {
  AccessSourceList,
  orderedSources,
  type RoleReason,
  type SourceKind,
} from "@/components/access/AccessSource";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { FilterPills } from "@/components/ui/Select";
import { WithheldInline } from "@/components/ui/Withheld";
import { PageHeader } from "@/components/ui/PageHeader";
import { useCrumb } from "@/lib/page-crumb";
import { peopleHref } from "@/lib/people-filters";
import { useRoleMembers, type RoleMember } from "@/lib/queries/useRoleMembers";
import { formatShortDate, humanizeKey } from "@/lib/format";

type Filter = "all" | SourceKind;

/**
 * E2 · Role → members. "Who can currently use the laser cutter?"
 *
 * Every row carries its source, and every row's action is named after its own
 * source. There is never a generic "Revoke role": a person can hold one role
 * three ways, so a generic action is ambiguous at best and destructive at
 * worst.
 */
export default function RoleMembersPage({
  params,
}: {
  params: Promise<{ id: string; key: string }>;
}) {
  const { id, key } = use(params);
  const roleKey = decodeURIComponent(key);
  const members = useRoleMembers(id, roleKey);

  const [filter, setFilter] = useState<Filter>("all");
  const [removal, setRemoval] = useState<(Removal & { userName: string }) | null>(null);

  const view = members.data;
  useCrumb(view ? `${view.project_name} / ${roleKey}` : null);

  const rows = useMemo(() => {
    const all = view?.members ?? [];
    if (filter === "all") return all;
    return all.filter((member) => member.reasons.some((reason) => reason.kind === filter));
  }, [view, filter]);

  return (
    <div className="flex flex-col gap-[22px]">
      <PageHeader
        eyebrow={view?.project_name ?? id}
        title={view?.display_name || humanizeKey(roleKey)}
        meta={
          <span className="flex flex-wrap items-center gap-3">
            <Mono className="rounded-pill border border-line-strong px-3 py-1 text-[14px] text-muted">
              {roleKey}
            </Mono>
            {view?.group && <span className="text-[13.5px] text-faint">Group: {view.group}</span>}
            {view?.cloned_from && (
              <span className="text-[13.5px] text-faint">cloned from {view.cloned_from}</span>
            )}
          </span>
        }
        actions={
          // The only outbound link this page has, and deliberately one-way.
          // This page stays read-only and source-aware: it knows WHY each
          // person holds the role, which a people list cannot. Adding people is
          // the one thing it can't answer from what it knows, so that — and
          // only that — hands off to People in bulk mode, pre-armed with this
          // project and role.
          <ButtonLink href={peopleHref({ project: id, role: roleKey }, { bulk: "1" })} variant="accent">
            Add people to this role
          </ButtonLink>
        }
      />

      <Card>
        <div className="flex flex-wrap items-center gap-3 px-5 py-4">
          <span className="type-card-title">
            {(view?.members.length ?? 0) === 1
              ? "1 person holds this role"
              : `${view?.members.length ?? 0} people hold this role`}
          </span>
          {/* Stated before the list rather than left to be discovered in it.
              "Forty people hold this role" is the sentence an operator acts on,
              and it is false for however many of them are holding it with
              something taken away. Not a filter pill: those partition by SOURCE,
              and a carve-out is orthogonal to how somebody came to hold it. */}
          {view?.withheld_unavailable ? (
            // A zero the page cannot stand behind. Saying "none" from a read
            // that did not happen is the failure this column exists to close,
            // reproduced on the screen that closes it.
            <span className="text-[13.5px] text-warn-text">
              Holds could not be read — this list does not say who has something
              withheld
            </span>
          ) : (
            (view?.withheld_count ?? 0) > 0 && (
              <span className="text-[13.5px] text-warn-text">
                {view?.withheld_count === 1
                  ? "1 of them has something withheld"
                  : `${view?.withheld_count} of them have something withheld`}
              </span>
            )
          )}
          <span className="flex-1" />
          <span className="text-[13.5px] text-faint">Filter by source:</span>
          <FilterPills<Filter>
            label="Filter by access source"
            value={filter}
            onChange={setFilter}
            options={[
              { value: "all", label: "All" },
              { value: "direct", label: `Direct ${view?.direct_count ?? 0}` },
              { value: "bundle", label: `Bundle ${view?.bundle_count ?? 0}` },
              { value: "mapping", label: `Automatic ${view?.automatic_count ?? 0}` },
            ]}
          />
        </div>

        <ListStates
          isLoading={members.isLoading}
          error={members.error}
          isEmpty={rows.length === 0}
          onRetry={() => members.refetch()}
          errorTitle="Couldn't load role members."
          skeleton={<RowSkeleton rows={5} label="Loading role members" />}
          empty={
            filter === "all" ? (
              <EmptyState
                title="Nobody holds this role yet."
                guidance="Grant it to someone directly, or add it to a bundle."
                action={{ label: "Go to People", href: "/users" }}
              />
            ) : (
              <EmptyState
                title={`Nobody holds it that way.`}
                guidance="Other sources may still carry it — clear the filter to see everyone."
                action={{ label: "Show everyone", onClick: () => setFilter("all") }}
              />
            )
          }
        >
          {rows.map((member) => (
            <MemberRow
              key={member.user.id}
              member={member}
              projectId={id}
              projectName={view?.project_name ?? id}
              roleKey={roleKey}
              onRemove={setRemoval}
            />
          ))}
        </ListStates>
      </Card>

      <RemovalDialog
        removal={removal}
        userId={removal?.userId}
        userName={removal?.userName}
        onClose={() => setRemoval(null)}
      />
    </div>
  );
}

function MemberRow({
  member,
  projectId,
  projectName,
  roleKey,
  onRemove,
}: {
  member: RoleMember;
  projectId: string;
  projectName: string;
  roleKey: string;
  onRemove: (removal: Removal & { userName: string }) => void;
}) {
  const sources = orderedSources(member.reasons);
  const strongest = sources[0];

  const open = (kind: SourceKind) =>
    onRemove({
      projectId,
      projectName,
      roleKey,
      sources: reorder(sources, kind),
      grantId: member.grant_id,
      userId: member.user.id,
      userName: member.user.name,
    });

  return (
    <div className="row-divider flex items-center gap-[18px] px-5 py-3">
      <Avatar name={member.user.name} />
      <Link
        href={`/users/${member.user.id}`}
        className="w-[210px] shrink-0 truncate text-[15px] font-semibold hover:underline"
      >
        {member.user.name}
      </Link>
      <div className="w-[170px] shrink-0 truncate text-[14px] text-muted">
        {member.user.title || member.user.team || "—"}
      </div>
      <div className="min-w-0 flex-1">
        <AccessSourceList reasons={member.reasons} />
        {/* On the row, under the source, because it modifies what the source
            means: "holds it via the maker bundle" and "holds it via the maker
            bundle, with the share withheld" are different facts about the same
            person, and only one of them is what the operator is looking at. */}
        {member.withheld && member.withheld.length > 0 && (
          <WithheldInline
            items={member.withheld.map((held) => ({
              field: held.field,
              value: held.value,
              reason: held.reason,
              target: held.target,
              actorId: held.actor_id,
            }))}
          />
        )}
      </div>
      <div className="w-[120px] shrink-0 text-[13.5px] text-faint">
        {member.expires
          ? `Expires ${formatShortDate(member.expires)}`
          : member.since
            ? `Since ${formatShortDate(member.since)}`
            : "No expiry"}
      </div>

      {/* The action is named after the thing being removed. Where nothing can
          be removed here, the row offers the rule instead of a dead control. */}
      {strongest?.kind === "mapping" ? (
        <button
          type="button"
          onClick={() => open("mapping")}
          className="shrink-0 text-[13px] font-semibold text-accent-text hover:underline"
        >
          Open the rule →
        </button>
      ) : strongest?.kind === "bundle" ? (
        <Button variant="danger" size="sm" onClick={() => open("bundle")}>
          Remove bundle assignment
        </Button>
      ) : (
        <Button variant="danger" size="sm" onClick={() => open("direct")}>
          Remove direct access
        </Button>
      )}
    </div>
  );
}

/** Puts the chosen source first so the dialog opens on the one that was clicked. */
function reorder(sources: RoleReason[], kind: SourceKind): RoleReason[] {
  const chosen = sources.filter((source) => source.kind === kind);
  const rest = sources.filter((source) => source.kind !== kind);
  return [...chosen, ...rest];
}

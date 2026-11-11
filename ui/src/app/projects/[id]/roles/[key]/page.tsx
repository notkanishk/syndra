"use client";

import { useSchemaSource } from "@/lib/queries/useFieldSchema";
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
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Notice } from "@/components/ui/Notice";
import { ProjectName, projectLabel } from "@/components/names";
import { Card, CardHeader } from "@/components/ui/Card";
import { FilterPills } from "@/components/ui/Select";
import { WithheldInline } from "@/components/ui/Withheld";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilterBar } from "@/components/ui/FilterBar";
import { useIsAdvanced } from "@/lib/ui-view";
import { useCrumb } from "@/lib/page-crumb";
import { sortByName } from "@/lib/sort";
import { driftHref } from "@/lib/drift-filters";
import { peopleHref } from "@/lib/people-filters";
import { useRoleMembers, type RoleMember } from "@/lib/queries/useRoleMembers";
import { formatShortDate } from "@/lib/format";
import { roleName } from "@/lib/names";
import { fromARead, holds, standingTone, standingWords } from "@/lib/standing";

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
  const advanced = useIsAdvanced();
  const members = useRoleMembers(id, roleKey);

  const [filter, setFilter] = useState<Filter>("all");
  const [removal, setRemoval] = useState<(Removal & { userName: string }) | null>(null);

  const view = members.data;
  useCrumb(view ? `${projectLabel(view.project_name)} / ${roleKey}` : null);

  const rows = useMemo(() => {
    const all = sortByName(view?.members ?? [], (member) => member.user.name, (member) => member.user.id);
    if (filter === "all") return all;
    return all.filter((member) => member.reasons.some((reason) => reason.kind === filter));
  }, [view, filter]);

  const observedOnly = sortByName(view?.observed_only ?? [], (m) => m.user.name, (m) => m.user.id);
  // Holding is observed: a record Zitadel does not show explains, it does not
  // hold. The backend's standing says who holds (a role being removed is still
  // held); Zitadel never read is not a count, the way the roles index says it.
  const holdCount =
    [...(view?.members ?? []), ...observedOnly].filter((member) => holds(member.standing)).length;
  // Each member's standing stands on their own read or the org sweep, so the
  // count is "not checked" only while nobody's has been read at all.
  const notChecked = !view?.observation?.read_at && !(view?.members ?? []).some((m) => fromARead(m.standing));

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        eyebrow={
          <ProjectName id={id} name={view?.project_name} />
        }
        title={roleName(view?.display_name, roleKey)}
        lede="Everyone who holds this role, and how each came to hold it: directly, through a bundle, or by an automatic rule."
        meta={
          <span className="flex flex-wrap items-center gap-3">
            {advanced && <Mono className="text-faint">{roleKey}</Mono>}
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
          <ButtonLink href={peopleHref({}, { bulk: "1", give_project: id, give_role: roleKey })} variant="accent">
            Add people to this role
          </ButtonLink>
        }
      />

      {view?.withheld_unavailable && (
        // A zero the page cannot stand behind. Saying "none" from a read that did not happen is
        // the failure this column exists to close, reproduced on the screen that closes it.
        <Notice tone="warn" title="Holds could not be read." next="Reload the page in a minute to read them again.">
          This list does not say who has something withheld.
        </Notice>
      )}

      {/* One control, so it stays on the row. It used to sit inside the card
          header behind a literal "Filter by source:" label, which is the only
          place in the product a filter introduced itself in prose. */}
      <FilterBar
        leading={
          <FilterPills<Filter>
            label="Filter by access source"
            value={filter}
            onChange={setFilter}
            options={[
              { value: "all", label: "All" },
              // No counts until they are read: "Direct 0" while loading is a fabricated zero.
              { value: "direct", label: view ? `Direct ${view.direct_count}` : "Direct" },
              { value: "bundle", label: view ? `Bundle ${view.bundle_count}` : "Bundle" },
              { value: "mapping", label: view ? `Automatic ${view.automatic_count}` : "Automatic" },
            ]}
          />
        }
      />

      <Card>
        {/* Syndra's own record plus whoever Zitadel shows holding it with no Syndra record at all:
            "holding is observed", and a count that dropped the second half would undercount who can
            actually use this. Not a filter pill: those partition by SOURCE, and a carve-out is
            orthogonal to how somebody came to hold it. Nothing while loading (a fabricated zero). */}
        {view && (
          <CardHeader
            title={
              notChecked
                ? "Not checked yet"
                : holdCount === 1
                  ? "1 person holds this role"
                  : `${holdCount} people hold this role`
            }
            note={
              !view.withheld_unavailable && view.withheld_count > 0 ? (
                <span className="text-warn-text">
                  {view.withheld_count === 1
                    ? "1 of them has something withheld"
                    : `${view.withheld_count} of them have something withheld`}
                </span>
              ) : undefined
            }
          />
        )}

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
                guidance="Give it to someone directly, or add it to a bundle."
                action={{ label: "Go to People", href: "/users" }}
              />
            ) : (
              <EmptyState
                title={`Nobody holds it that way.`}
                guidance="Other sources may still carry it. Clear the filter to see everyone."
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
              projectName={projectLabel(view?.project_name)}
              roleKey={roleKey}
              onRemove={setRemoval}
            />
          ))}
        </ListStates>
      </Card>

      {observedOnly.length > 0 && (
        <Card>
          <CardHeader
            title="In Zitadel only"
            action={<ArrowLink href={driftHref({ project: id, role: roleKey })}>Review in drift</ArrowLink>}
          />
          {observedOnly.map(({ user: person, standing }) => (
            <Link
              data-row
              key={person.id}
              href={`/users/${person.id}`}
              className="row-divider flex min-h-[60px] items-center gap-3 px-5 py-3 hover-tint"
            >
              <Avatar name={person.name} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[15px] font-semibold">{person.name}</span>
                <span className="block truncate text-[13px] text-faint">{person.email}</span>
                {/* Held is why they are listed; only a removal on its way is news. */}
                {standing && standing !== "held" && (
                  <span
                    className={`block text-[12.5px] ${
                      standingTone(standing) === "warn" ? "font-semibold text-warn-text" : "text-faint"
                    }`}
                  >
                    {standingWords(standing, "operator")}
                  </span>
                )}
              </span>
            </Link>
          ))}
        </Card>
      )}

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
  const schemaOf = useSchemaSource(true);
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
    <div className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:items-center tablet:gap-[18px]">
      <Avatar name={member.user.name} />
      <Link
        href={`/users/${member.user.id}`}
        className="w-[210px] shrink-0 truncate text-[15px] font-semibold link-hover"
      >
        {member.user.name}
      </Link>
      <div
        className="w-[170px] shrink-0 truncate text-[14px] text-muted"
        title={member.user.title || member.user.team || member.user.email}
      >
        {member.user.title || member.user.team || member.user.email}
      </div>
      <div className="min-w-0 flex-1">
        {/* Held is the healthy case, and an unread Zitadel is said once, in
            the card's title, not on every row. */}
        {member.standing && member.standing !== "held" && (
          <span
            className={`mb-0.5 block text-[12.5px] ${
              standingTone(member.standing) === "warn" ? "font-semibold text-warn-text" : "text-faint"
            }`}
          >
            {standingWords(member.standing, "operator")}
          </span>
        )}
        <AccessSourceList reasons={member.reasons} />
        {/* On the row, under the source, because it modifies what the source
            means: "holds it via the maker bundle" and "holds it via the maker
            bundle, with the share withheld" are different facts about the same
            person, and only one of them is what the operator is looking at. */}
        {member.withheld && member.withheld.length > 0 && (
          <WithheldInline
            schemaOf={schemaOf}
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
          ? `Ends ${formatShortDate(member.expires)}`
          : member.since
            ? `Since ${formatShortDate(member.since)}`
            : "No end date"}
      </div>

      {/* The action is named after the thing being removed. Where nothing can
          be removed here, the row offers the rule instead of a dead control. */}
      {strongest?.kind === "mapping" ? (
        <Button variant="outline" size="sm" onClick={() => open("mapping")}>
          Open the rule
        </Button>
      ) : strongest?.kind === "bundle" ? (
        <Button variant="danger" size="sm" onClick={() => open("bundle")}>
          Remove bundle assignment
        </Button>
      ) : (
        <Button variant="danger" size="sm" onClick={() => open("direct")}>
          Revoke direct access
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

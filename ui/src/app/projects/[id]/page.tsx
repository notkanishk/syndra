"use client";

import Link from "next/link";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { use, useMemo, useState } from "react";

import { ProjectClaimDefault } from "@/components/projects/ProjectClaimDefault";
import { CreateRoleDialog } from "@/components/roles/CreateRoleDialog";
import { EmptyState, ErrorState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { Skeleton, useTintAfterSkeleton } from "@/components/ui/Skeleton";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { useCrumb } from "@/lib/page-crumb";
import { sortByName } from "@/lib/sort";
import { useApplications } from "@/lib/queries/useApplications";
import { useSeededProjectName } from "@/lib/queries/seed";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";
import { Tinted } from "@/components/ui/Unfold";
import { useIsAdvanced } from "@/lib/ui-view";

/**
 * E1 detail · a project's roles. Each row's member count is the link into E2 —
 * "who can currently use this?" is one click from "what roles exist here?".
 *
 * Descriptions are shown in FULL, never truncated to a tooltip: "can cut
 * unsupervised" versus "may enter and watch" is the entire decision an operator
 * is making, and a role key alone never conveys it.
 */
export default function ProjectDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const projects = useProjects();
  const roles = useGlobalRoleCatalog();
  const apps = useApplications();
  const advanced = useIsAdvanced();

  const project = (projects.data ?? []).find(
    (entry) => entry.project.id === id,
  );
  // The name from the list the reader came from, before this page's own read lands (A-19 P1).
  const seeded = useSeededProjectName(id);
  useCrumb(project?.project.name ?? seeded);
  // The title that lands after its skeleton tints in, not snaps (S7).
  const titleTint = useTintAfterSkeleton(projects.isLoading && !project);

  const projectRoles = useMemo(
    () =>
      sortByName(
        (roles.data ?? []).filter((role) => role.project_id === id),
        (role) => role.display_name || humanizeKey(role.role_key),
        (role) => role.role_key,
      ),
    [roles.data, id],
  );
  const served = sortByName(
    (apps.data ?? []).filter((entry) => entry.application.project_id === id),
    (entry) => entry.application.name,
    (entry) => entry.application.id,
  );
  const [creating, setCreating] = useState(false);

  // Only once the project itself is in hand. `member_count ?? 0` rendered
  // "0 people" for a project still loading — and for ever, for an id that
  // resolves to nothing — which is a fabricated zero, not a count.
  const people = project
    ? holdersLine(
        project.member_count,
        project.confirmed_member_count,
        project.observed_member_count,
      )
    : null;
  const readState: ReadState = {
    readAt: project?.observation?.read_at,
    current: project?.observation ? project.observation.current : undefined,
    truncated: project?.observation?.truncated,
  };

  // A failed list says nothing about whether the project exists: "isn't listed"
  // is for a list that answered without it.
  if (projects.error && !project) {
    return (
      <ErrorState
        title="Couldn't load this project."
        error={projects.error}
        onRetry={() => projects.refetch()}
      />
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        // Never the id: while the list loads the title is a skeleton, and a project the
        // list doesn't carry is said so in words.
        title={
          project ? (
            <span className={titleTint}>{project.project.name}</span>
          ) : seeded && projects.isLoading ? (
            <span>{seeded}</span>
          ) : projects.isLoading ? (
            <Skeleton className="h-7 w-56" aria-label="Loading project" />
          ) : (
            "That project isn't listed"
          )
        }
        lede={
          <>
            The roles this project offers and who holds each.
            {/* Each app it reads is a link to the app (RULES: every name with a page of its own links there). */}
            <span className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1">
              {served.length > 0 ? (
                <>
                  <span>Read by</span>
                  {served.map((entry) => (
                    <ArrowLink key={entry.application.id} href={`/applications/${entry.application.id}`}>
                      {entry.application.name}
                    </ArrowLink>
                  ))}
                </>
              ) : (
                <span>No app reads this yet.</span>
              )}
            </span>
          </>
        }
        // Nothing until the project is in hand: "0 roles" would be a count of nothing, said as
        // though it were a fact about the project. The project id stays (owner Q6).
        meta={
          !project ? undefined : (
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span>
                {people ? `${people.headline} ${people.headline === "1" ? "person" : "people"} · ` : ""}
                {projectRoles.length} {projectRoles.length === 1 ? "role" : "roles"}
              </span>
              <Tinted show={advanced}>
                <CopyableValue inline value={id} label="Project ID" />
              </Tinted>
            </span>
          )
        }
        actions={
          project && (
            <Button variant="accent" onClick={() => setCreating(true)}>
              New role
            </Button>
          )
        }
      />

      {project && <ReadFreshness state={readState} subject="What Zitadel confirmed" />}

      <Card stale={readState.current === false}>
        <CardColumns>
          <span className="min-w-[200px] flex-1">Role</span>
          <span className="hidden w-[130px] shrink-0 desktop:block">Group</span>
          <span className="hidden w-[140px] shrink-0 tablet:block">When given</span>
          <span className="w-[110px] shrink-0 text-right">People</span>
        </CardColumns>

        <ListStates
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={projectRoles.length === 0}
          onRetry={() => roles.refetch()}
          errorTitle="Couldn't load this project's roles."
          skeleton={
            <RowSkeleton rows={4} avatar={false} label="Loading roles" />
          }
          empty={
            <EmptyState
              title="No roles listed in this project."
              guidance="Roles created directly in Zitadel may not be listed yet."
            />
          }
        >
          {projectRoles.map((role) => {
            const members = holdersLine(
              role.assigned_user_count,
              role.confirmed_user_count,
              role.observed_user_count,
            );
            return (
              <div
                key={role.role_key}
                className="row-divider flex flex-wrap items-start gap-[18px] px-5 py-3.5"
              >
                <div className="min-w-[min(220px,100%)] flex-1">
                  <div className="text-[15px] font-semibold">
                    <Link
                      href={`/projects/${id}/roles/${encodeURIComponent(role.role_key)}`}
                      className="link-hover"
                    >
                      {role.display_name || humanizeKey(role.role_key)}
                    </Link>
                    {/* Below tablet the When given column is gone; the mark
                        stays, after the name, and says it on tap or focus. */}
                    {role.waits_for_send && (
                      <span className="ml-2 tablet:hidden">
                        <WaitsMark />
                      </span>
                    )}{" "}
                    <Mono className="font-normal text-faint">
                      {role.role_key}
                    </Mono>
                  </div>
                  {role.description && (
                    <p className="mt-1 max-w-[80ch] text-[13.5px] leading-[1.5] text-muted [text-wrap:pretty]">
                      {role.description}
                    </p>
                  )}
                  {role.cloned_from_role && (
                    <p className="mt-1 text-[12.5px] text-faint">
                      cloned from {role.cloned_from_project} /{" "}
                      {role.cloned_from_role}
                    </p>
                  )}
                </div>

                <span className="hidden w-[130px] shrink-0 text-[13.5px] text-muted [text-wrap:pretty] desktop:block">
                  {role.group || "—"}
                </span>

                {/* Read-only (N16): set only in Roles that wait. */}
                <span className="hidden w-[140px] shrink-0 text-[13.5px] tablet:block">
                  {role.waits_for_send ? (
                    <WaitsMark
                      withLabel
                      className="[&>span:nth-child(2)]:text-ink"
                    />
                  ) : (
                    <span className="text-faint">Straight away</span>
                  )}
                </span>

                <span className="flex w-[110px] shrink-0 flex-col items-end gap-0.5 text-[15px]">
                  <span>{members.headline}</span>
                  {members.note && (
                    <span
                      className={`text-[12.5px] font-normal ${holdersToneClass[members.tone]}`}
                    >
                      {members.note}
                    </span>
                  )}
                </span>
              </div>
            );
          })}
        </ListStates>
      </Card>

      {projectRoles.length > 0 && (
        <div className="px-5">
          <ArrowLink href="/changes/roles-that-wait">Set in Roles that wait</ArrowLink>
        </div>
      )}

      {/* The project default's one write place (A-14 §7); each app page reads it. */}
      {project && <ProjectClaimDefault projectId={id} />}

      {creating && (
        <CreateRoleDialog
          pinnedProjectId={id}
          onClose={() => setCreating(false)}
        />
      )}
    </div>
  );
}

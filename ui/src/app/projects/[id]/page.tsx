"use client";

import Link from "next/link";
import { use, useMemo, useState } from "react";

import { CreateRoleDialog } from "@/components/roles/CreateRoleDialog";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useCrumb } from "@/lib/page-crumb";
import { useApplications } from "@/lib/queries/useApplications";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * E1 detail · a project's roles. Each row's member count is the link into E2 —
 * "who can currently use this?" is one click from "what roles exist here?".
 *
 * Descriptions are shown in FULL, never truncated to a tooltip: "can cut
 * unsupervised" versus "may enter and watch" is the entire decision an operator
 * is making, and a role key alone never conveys it.
 */
export default function ProjectDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const projects = useProjects();
  const roles = useGlobalRoleCatalog();
  const apps = useApplications();

  const project = (projects.data ?? []).find((entry) => entry.project.id === id);
  useCrumb(project?.project.name);

  const projectRoles = useMemo(
    () => (roles.data ?? []).filter((role) => role.project_id === id),
    [roles.data, id],
  );
  const served = (apps.data ?? []).filter((entry) => entry.application.project_id === id);
  const [creating, setCreating] = useState(false);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        eyebrow="Projects"
        title={project?.project.name ?? id}
        meta={
          <span className="flex flex-wrap items-center gap-2">
            <span>
              {project?.member_count ?? 0} people · {projectRoles.length}{" "}
              {projectRoles.length === 1 ? "role" : "roles"}
              {served.length > 0
                ? ` · serves ${served.map((entry) => entry.application.name).join(" and ")}`
                : " · no app reads this yet"}
            </span>
            <Mono className="text-faint">{id}</Mono>
          </span>
        }
        actions={
          <>
            {served.length > 0 && (
              <ButtonLink href={`/applications/${served[0].application.id}`}>
                Token format
              </ButtonLink>
            )}
            {/*
              The natural place to add a role to this project is this page. It
              used to exist only on the cross-project index, so the closest
              affordance here was "Check upstream" — which creates the role in
              the identity provider with no local row, and it comes back as
              unexplained access.
            */}
            <Button variant="accent" onClick={() => setCreating(true)}>
              New role
            </Button>
          </>
        }
      />

      <Card>
        <CardColumns>
          <span className="flex-1">Role</span>
          <span className="w-[130px]">Group</span>
          <span className="w-[96px] text-right">Members</span>
        </CardColumns>

        <ListStates
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={projectRoles.length === 0}
          onRetry={() => roles.refetch()}
          errorTitle="Couldn't load this project's roles."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Loading roles" />}
          empty={
            <EmptyState
              title="No roles listed in this project."
              guidance="Create one here, or check the identity provider — roles created directly there may not be listed yet."
              action={{ label: "Create a role", onClick: () => setCreating(true) }}
            />
          }
        >
          {projectRoles.map((role) => (
            <div
              key={role.role_key}
              className="row-divider flex items-start gap-[18px] px-5 py-3.5"
            >
              <div className="min-w-0 flex-1">
                <div className="text-[15px] font-semibold">
                  {role.display_name || humanizeKey(role.role_key)}{" "}
                  <Mono className="font-normal text-faint">{role.role_key}</Mono>
                </div>
                {role.description && (
                  <p className="mt-1 max-w-[80ch] text-[13.5px] leading-[1.5] text-ink/50">
                    {role.description}
                  </p>
                )}
                {role.cloned_from_role && (
                  <p className="mt-1 text-[12.5px] text-faint">
                    cloned from {role.cloned_from_project} / {role.cloned_from_role}
                  </p>
                )}
              </div>

              <span className="w-[130px] shrink-0 truncate text-[13.5px] text-muted">
                {role.group || "—"}
              </span>

              <Link
                href={`/projects/${id}/roles/${encodeURIComponent(role.role_key)}`}
                className="w-[96px] shrink-0 text-right text-[15px] font-semibold text-accent-text"
              >
                {role.assigned_user_count} →
              </Link>
            </div>
          ))}
        </ListStates>
      </Card>

      {creating && <CreateRoleDialog pinnedProjectId={id} onClose={() => setCreating(false)} />}
    </div>
  );
}

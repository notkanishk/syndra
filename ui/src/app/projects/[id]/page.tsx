"use client";

import Link from "next/link";
import { use, useMemo } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useCrumb } from "@/lib/page-crumb";
import { useApplications } from "@/lib/queries/useApplications";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * A project's roles. Each row's member count is the link into E2 — "who can
 * currently use this?" is one click from "what roles exist here?".
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

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        eyebrow="Project"
        title={project?.project.name ?? id}
        meta={
          <span className="flex flex-wrap items-center gap-2">
            <span>
              {projectRoles.length} {projectRoles.length === 1 ? "role" : "roles"} ·{" "}
              {project?.member_count ?? 0} people
            </span>
            {served.map((entry) => (
              <span
                key={entry.application.id}
                className="rounded-pill bg-tint-2 px-2.5 py-1 text-[12.5px]"
              >
                {entry.application.name}
              </span>
            ))}
          </span>
        }
      />

      <Card>
        <CardColumns>
          <span className="flex-1">Role</span>
          <span className="w-[150px]">Group</span>
          <span className="w-[240px]">Description</span>
          <span className="w-[90px] text-right">Members</span>
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
              title="No MkAuth-managed roles in this project."
              guidance="Roles created directly in the identity provider aren't listed here yet."
            />
          }
        >
          {projectRoles.map((role) => (
            <Link
              key={role.role_key}
              href={`/projects/${id}/roles/${encodeURIComponent(role.role_key)}`}
              className="row-divider flex items-center gap-[18px] px-5 py-3.5 transition-colors hover:bg-[var(--hover)]"
            >
              <span className="min-w-0 flex-1 truncate text-[15px] font-semibold">
                {role.display_name || humanizeKey(role.role_key)}{" "}
                <Mono className="font-normal text-faint">{role.role_key}</Mono>
              </span>
              <span className="w-[150px] truncate text-[13.5px] text-muted">
                {role.source === "mkauth" ? "MkAuth-managed" : role.source}
              </span>
              <span className="w-[240px] truncate text-[13.5px] text-muted">
                {role.description || "—"}
              </span>
              <span className="w-[90px] text-right text-[15px]">{role.assigned_user_count}</span>
            </Link>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

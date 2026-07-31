"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * E2 index · /roles — the cross-project role index, and the landing target for
 * the rail's Roles item.
 *
 * Project is the FIRST column and never collapses: the same key in two
 * projects means two different things, and a list that leads with the key
 * invites treating them as one role.
 */
export default function RolesPage() {
  const roles = useGlobalRoleCatalog();
  const [project, setProject] = useState("");
  const [group, setGroup] = useState("");

  const all = useMemo(() => roles.data ?? [], [roles.data]);

  const projects = useMemo(
    () => Array.from(new Set(all.map((role) => role.project_name || role.project_id))).sort(),
    [all],
  );
  const groups = useMemo(
    () => Array.from(new Set(all.map((role) => role.source).filter(Boolean))).sort(),
    [all],
  );

  const rows = all.filter(
    (role) =>
      (!project || (role.project_name || role.project_id) === project) &&
      (!group || role.source === group),
  );

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Roles"
        meta="across every project"
        actions={
          <>
            <Select
              value={project}
              onChange={(event) => setProject(event.target.value)}
              aria-label="Filter by project"
              className="w-[190px]"
            >
              <option value="">All projects</option>
              {projects.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </Select>
            <Select
              value={group}
              onChange={(event) => setGroup(event.target.value)}
              aria-label="Filter by group"
              className="w-[170px]"
            >
              <option value="">All groups</option>
              {groups.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </Select>
          </>
        }
      />

      {/*
        Required scope notice. GET /api/v1/roles returns only roles created
        through MkAuth. Stating that is not pedantry: a silently partial list
        is how somebody concludes a role doesn't exist and creates a duplicate.
      */}
      <div className="accent-note flex items-start gap-3 px-[18px] py-3.5">
        <span
          aria-hidden
          className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-accent-soft text-[12px] font-bold text-accent-text"
        >
          i
        </span>
        <div className="text-[14px] leading-[1.55] text-ink/[.78]">
          <strong className="font-semibold text-ink">Showing MkAuth-managed roles only.</strong>{" "}
          Roles created directly in the identity provider aren&rsquo;t listed yet.{" "}
          <Link href="/zitadel" className="font-semibold text-accent-text">
            Check a project&rsquo;s roles in the identity provider →
          </Link>
        </div>
      </div>

      <Card>
        <CardColumns>
          <span className="w-[180px]">Project</span>
          <span className="flex-1">Role</span>
          <span className="w-[130px]">Group</span>
          <span className="w-[80px] text-right">Members</span>
        </CardColumns>

        <ListStates
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={rows.length === 0}
          onRetry={() => roles.refetch()}
          errorTitle="Couldn't load the role index."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading roles" />}
          empty={
            <EmptyState
              title="No roles match those filters."
              guidance="Clear a filter, or check the identity provider for roles MkAuth didn't create."
              action={{ label: "Clear filters", onClick: () => { setProject(""); setGroup(""); } }}
            />
          }
        >
          {rows.map((role) => (
            <Link
              key={`${role.project_id}:${role.role_key}`}
              href={`/projects/${role.project_id}/roles/${encodeURIComponent(role.role_key)}`}
              className="row-divider flex items-center gap-[18px] px-5 py-3 transition-colors hover:bg-[var(--hover)]"
            >
              <span className="w-[180px] truncate text-[14.5px] text-muted">
                {role.project_name || role.project_id}
              </span>
              <span className="min-w-0 flex-1 truncate text-[15px] font-semibold">
                {role.display_name || humanizeKey(role.role_key)}{" "}
                <Mono className="font-normal text-faint">{role.role_key}</Mono>
              </span>
              <span className="w-[130px] truncate text-[13.5px] text-muted">
                {role.source === "mkauth" ? "MkAuth-managed" : role.source}
              </span>
              <span className="w-[80px] text-right text-[15px]">{role.assigned_user_count}</span>
            </Link>
          ))}
        </ListStates>
      </Card>

      <p className="max-w-[900px] text-[14px] leading-[1.55] text-faint">
        Same key, two projects, two different things — which is why the project column is first and
        never collapses.
      </p>
    </div>
  );
}

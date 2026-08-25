"use client";

import Link from "next/link";
import { useMemo } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useApplications } from "@/lib/queries/useApplications";
import { useProjects } from "@/lib/queries/useProjects";

/**
 * E1 · Projects. A project is a boundary that owns roles; an app is a thing
 * that receives a token. They are NOT one-to-one — Badge Reader reads four
 * projects and Studio Access feeds two apps — so "Apps served" is a column of
 * pills and never a single value.
 */
export default function ProjectsPage() {
  const projects = useProjects();
  const apps = useApplications();

  const appsByProject = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const entry of apps.data ?? []) {
      const list = map.get(entry.application.project_id) ?? [];
      list.push(entry.application.name);
      map.set(entry.application.project_id, list);
    }
    return map;
  }, [apps.data]);

  const rows = projects.data ?? [];
  const servedCount = new Set((apps.data ?? []).map((a) => a.application.id)).size;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Projects"
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "boundary" : "boundaries"} · ${servedCount} apps served`
            : undefined
        }
      />

      <Card>
        <CardColumns>
          <span className="min-w-0 flex-1">Project</span>
          <span className="w-[70px] shrink-0 text-right">People</span>
          <span className="w-[60px] shrink-0 text-right">Roles</span>
          <span className="w-[240px]">Apps served</span>
        </CardColumns>

        <ListStates
          isLoading={projects.isLoading}
          error={projects.error}
          isEmpty={rows.length === 0}
          onRetry={() => projects.refetch()}
          errorTitle="Couldn't load projects."
          skeleton={<RowSkeleton rows={5} avatar={false} label="Loading projects" />}
          empty={
            <EmptyState
              title="No projects yet."
              guidance="A project appears here once it exists in the identity provider."
            />
          }
        >
          {rows.map((entry) => (
            <Link
              key={entry.project.id}
              href={`/projects/${entry.project.id}`}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 motion-tint hover:bg-[var(--hover)] tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              {/* The project's name, and — when there are no roles — the fact
                  that nothing in it can be granted. The sentence lives HERE and
                  not in the Roles column, which is 60px wide and right-aligned:
                  a 43-character sentence in it wrapped to six lines and made
                  the row four times the height of its neighbours. This column
                  is the one with room. */}
              <span className="flex w-full min-w-0 flex-col gap-0.5 tablet:flex-1">
                <span className="truncate text-[15.5px] font-semibold">
                  {entry.project.name}
                </span>
                {entry.active_role_keys.length === 0 && (
                  <span className="truncate text-[13px] text-faint">
                    No roles yet — nothing here can be granted
                  </span>
                )}
              </span>
              <span className="shrink-0 text-[15px] tablet:w-[70px] tablet:text-right">
                {entry.member_count}
                <span className="text-[13px] text-faint tablet:hidden">
                  {entry.member_count === 1 ? " person" : " people"}
                </span>
              </span>
              {/* A project with no roles is not a small number, it is a
                  different fact: nothing in it can be granted to anybody. The
                  count stays a count so the column reads as one, and the fact
                  is said beside the name, where there is width to say it. */}
              <span className="shrink-0 text-[15px] tablet:w-[60px] tablet:text-right">
                {entry.active_role_keys.length}
                <span className="text-[13px] text-faint tablet:hidden">
                  {entry.active_role_keys.length === 1 ? " role" : " roles"}
                </span>
              </span>
              <span className="flex w-full flex-wrap gap-1.5 tablet:w-[240px]">
                {(appsByProject.get(entry.project.id) ?? []).map((name) => (
                  <span
                    key={name}
                    className="rounded-pill bg-tint-2 px-2.5 py-1 text-[12.5px]"
                  >
                    {name}
                  </span>
                ))}
                {!appsByProject.has(entry.project.id) && (
                  <span className="text-[13px] text-faint">No app reads this yet</span>
                )}
              </span>
            </Link>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

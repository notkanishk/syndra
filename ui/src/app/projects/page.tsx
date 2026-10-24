"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { useApplications } from "@/lib/queries/useApplications";
import { useProjects, type ProjectSummaryRow } from "@/lib/queries/useProjects";

/**
 * E1 · Projects. A project is a boundary that owns roles; an app is a thing
 * that receives a token. They are NOT one-to-one — Badge Reader reads four
 * projects and Studio Access feeds two apps — so "Apps served" is a column of
 * pills and never a single value.
 *
 * Columns follow the board (8b): the name, the apps taking the width, then the
 * two counts last in their 110px seats. Below desktop Roles folds under the
 * name so the apps keep their room.
 */
type ProjectSort = "project" | "people";

const PROJECT_SORT: Record<ProjectSort, (row: ProjectSummaryRow) => string | number> = {
  project: (row) => row.project.name,
  // The number the row shows: what Zitadel holds, else what Syndra recorded.
  people: (row) => row.observed_member_count ?? row.member_count,
};

export default function ProjectsPage() {
  const projects = useProjects();
  const apps = useApplications();
  const [query, setQuery] = useState("");
  const [sort, setSort, view] = useSort<ProjectSort>("project");

  const appsByProject = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const entry of apps.data ?? []) {
      const list = map.get(entry.application.project_id) ?? [];
      list.push(entry.application.name);
      map.set(entry.application.project_id, list);
    }
    return map;
  }, [apps.data]);

  const all = useMemo(() => projects.data ?? [], [projects.data]);
  const needle = query.trim().toLowerCase();
  const rows = useMemo(
    () =>
      orderBy(
        all.filter(
          (entry) =>
            !needle ||
            entry.project.name.toLowerCase().includes(needle) ||
            (appsByProject.get(entry.project.id) ?? []).some((name) =>
              name.toLowerCase().includes(needle),
            ),
        ),
        view.sort,
        PROJECT_SORT,
      ),
    [all, needle, appsByProject, view.sort],
  );
  // A search drops projects: they fade where they were and the gap settles after (A-05 §2.10).
  const leaving = useLeavingRows(rows, (entry) => entry.project.id, !projects.isLoading && !projects.error, {
    narrowedBy: needle,
  });
  const servedCount = new Set((apps.data ?? []).map((a) => a.application.id)).size;

  // One basis for the whole page — every row carries an identical copy, so
  // the freshness is stated here once rather than once per row.
  const basis = all[0]?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Projects"
        lede="A project is a machine or area with its own set of roles — the Laser Cutter, the Studio. Each app checks a person's roles against one project. Open one to see its roles and who holds them."
        meta={
          all.length > 0
            ? `${all.length} ${all.length === 1 ? "project" : "projects"} · ${servedCount} apps served`
            : undefined
        }
      />

      <FilterBar
        search={
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search projects or apps…"
            aria-label="Search projects by name or app"
          />
        }
      />

      {/*
        People is what Syndra decided, not what Zitadel has confirmed — stated
        once here rather than once per row. See one-truth-many-checks.
      */}
      {all.length > 0 && (
        <ReadFreshness state={readState} subject="What Zitadel confirmed" className="-mt-1.5 px-[2px]" />
      )}

      <Card>
        <CardColumns>
          <span className="w-[250px] min-w-[170px]">
            <SortHead label="Project" name="project" sort={sort} onSort={setSort} />
          </span>
          <span className="min-w-[180px] flex-1">Apps served</span>
          <span className="hidden w-[110px] shrink-0 text-right desktop:block">Roles</span>
          <span className="w-[110px] shrink-0 text-right">
            <SortHead label="People" name="people" sort={sort} onSort={setSort} />
          </span>
        </CardColumns>

        <div className={view.className}>
        <ListStates
          isLoading={projects.isLoading}
          error={projects.error}
          isEmpty={leaving.rows.length === 0}
          onRetry={() => projects.refetch()}
          errorTitle="Couldn't load projects."
          skeleton={<RowSkeleton rows={5} avatar={false} label="Loading projects" />}
          empty={
            needle ? (
              <EmptyState
                title="No project matches that."
                guidance="Try part of a project's name, or the name of an app that reads it."
                action={{ label: "Clear the search", onClick: () => setQuery("") }}
              />
            ) : (
              <EmptyState
                title="No projects yet."
                guidance="A project appears here once it exists in Zitadel."
              />
            )
          }
        >
          {leaving.rows.map((entry) => {
            const people = holdersLine(
              entry.member_count,
              entry.confirmed_member_count,
              entry.observed_member_count,
            );
            const roleCount = entry.role_keys.length;
            return (
            <LeavingRow key={entry.project.id} {...leaving.props(entry.project.id)}>
            <Link
              href={`/projects/${entry.project.id}`}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 hover-tint tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              {/* The project's name, and — when there are no roles — the fact
                  that nothing in it can be granted. The sentence lives HERE and
                  not in the Roles column, which is a narrow right-aligned seat:
                  a 43-character sentence in it wrapped to six lines and made
                  the row four times the height of its neighbours. This column
                  is the one with room. Below desktop the Roles count folds in
                  as a line under the name. */}
              <span className="flex w-full min-w-0 flex-col gap-0.5 tablet:w-[250px] tablet:min-w-[170px] tablet:shrink-0">
                <span className="truncate text-[15.5px] font-semibold">
                  {entry.project.name}
                </span>
                {roleCount === 0 ? (
                  <span className="text-[13px] text-faint [text-wrap:pretty]">
                    No roles yet — nothing here can be granted
                  </span>
                ) : (
                  <span className="text-[13px] text-faint desktop:hidden">
                    {roleCount} {roleCount === 1 ? "role" : "roles"}
                  </span>
                )}
              </span>
              <span className="flex w-full flex-wrap gap-1.5 tablet:min-w-[180px] tablet:flex-1">
                {(appsByProject.get(entry.project.id) ?? []).map((name) => (
                  <Badge key={name}>{name}</Badge>
                ))}
                {!appsByProject.has(entry.project.id) && (
                  <span className="text-[13px] text-faint">No app reads this yet</span>
                )}
              </span>
              {/* A project with no roles is not a small number, it is a
                  different fact: nothing in it can be granted to anybody. The
                  count stays a count so the column reads as one, and the fact
                  is said beside the name, where there is width to say it. */}
              <span className="hidden text-right text-[15px] desktop:block desktop:w-[110px] desktop:shrink-0">
                {roleCount}
              </span>
              <span className="shrink-0 text-[15px] tablet:w-[110px] tablet:text-right">
                {people.headline}
                <span className="text-[13px] text-faint tablet:hidden">
                  {people.headline === "1" ? " person" : " people"}
                </span>
                {/* Recorded is what Syndra decided; this is what Zitadel shows
                    right now — never the same word, never the same line. */}
                {people.note && (
                  <span className={`block text-[12.5px] ${holdersToneClass[people.tone]}`}>
                    {people.note}
                  </span>
                )}
              </span>
            </Link>
            </LeavingRow>
            );
          })}
        </ListStates>
        </div>
      </Card>
    </div>
  );
}

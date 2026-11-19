"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { ProjectName } from "@/components/names";
import { projectLabel } from "@/components/names/Unresolved";
import { CreateRoleDialog } from "@/components/roles/CreateRoleDialog";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, Cells, NUMBER_COL, ROW, type Columns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilterBar } from "@/components/ui/FilterBar";
import { LeavingRow, useLeavingRows } from "@/lib/motion";
import { FilterPanel, FilterField } from "@/components/ui/FilterPanel";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { Select } from "@/components/ui/Select";
import { RunLabel, SortHead, orderBy, runStarts, useSort } from "@/components/ui/SortHead";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import { heldSortKey, holdersLine, holdersToneClass, peopleSuffix } from "@/lib/holders";
import { sortByName, sortNames } from "@/lib/sort";
import { roleName } from "@/lib/names";

type RoleSort = "project" | "role" | "group" | "people";

const ROLE_SORT: Record<RoleSort, (role: CatalogRole) => string | number> = {
  project: (role) => projectLabel(role.project_name),
  role: (role) => roleName(role.display_name, role.role_key),
  group: (role) => role.group || "",
  // The number the row shows: what Zitadel holds, else what Syndra recorded.
  people: (role) => heldSortKey(role.observed_user_count),
};

/**
 * E2 index · /roles — the cross-project role index, and the landing target for
 * the rail's Roles item.
 *
 * Project is the FIRST column: the same key in two projects means two different
 * things, and a list that leads with the key invites treating them as one role.
 * Sorted by it, a project's roles gather under one quiet label instead of
 * repeating its name row after row.
 */
/**
 * The columns, the headings' and every row's: project, role, group and what uses it (from desktop;
 * below it they fold into a line under the role), and People in the number seat. Shares held to a
 * minimum: fixed 180 + 130 + 150 and a 200 minimum came to 834px, past the 754px row at 1100, so
 * the headings wrapped while the rows ran on and the card cut the count to "2 u" (staging, 19 Nov).
 */
const ROLE_COLS: Columns = [
  "w-full min-w-0 tablet:min-w-[120px] tablet:flex-1",
  "w-full min-w-0 tablet:min-w-[160px] tablet:flex-[2_1_0%]",
  "hidden desktop:block desktop:min-w-[100px] desktop:flex-1",
  "hidden desktop:block desktop:min-w-[120px] desktop:flex-1",
  NUMBER_COL,
];

export default function RolesPage() {
  const roles = useGlobalRoleCatalog();
  const router = useRouter();
  const params = useSearchParams();
  const [project, setProject] = useState("");
  const [group, setGroup] = useState("");
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [sort, setSort, view] = useSort<RoleSort>("project", { list: "roles", keys: ["project", "role", "group", "people"] });

  // Unused lives in the URL, unlike the other two, because Home links straight
  // here with "N roles nobody holds" — a link that until now landed on an
  // unfiltered index and left the reader to find them.
  const unusedOnly = params.get("unused") === "1";

  // Alphabetical by role first: every sort keeps ties in this order, so a project's roles read A to Z.
  const all = useMemo(
    () => sortByName(roles.data ?? [], (role) => roleName(role.display_name, role.role_key), (role) => role.role_key),
    [roles.data],
  );

  // Keyed by id, labelled by name: two projects Syndra could not name are two
  // projects, not one "Unknown project", and an id is never the label.
  const projects = useMemo(() => {
    const byId = new Map(all.map((role) => [role.project_id, projectLabel(role.project_name)]));
    return sortByName(Array.from(byId, ([id, label]) => ({ id, label })), (p) => p.label);
  }, [all]);
  const groups = useMemo(
    () => sortNames(Array.from(new Set(all.map((role) => role.group).filter(Boolean))) as string[]),
    [all],
  );

  const needle = query.trim().toLowerCase();
  const rows = orderBy(
    all.filter(
      (role) =>
        (!project || role.project_id === project) &&
        (!group || role.group === group) &&
        (!unusedOnly || role.is_unused) &&
        (!needle ||
          [role.display_name, role.role_key, role.project_name]
            .filter(Boolean)
            .some((text) => text.toLowerCase().includes(needle))),
    ),
    view.sort,
    ROLE_SORT,
  );
  // A filter or a search drops roles: they fade where they were and the gap settles after (A-05 §2.10).
  const leaving = useLeavingRows(rows, (role) => `${role.project_id}:${role.role_key}`, !roles.isLoading && !roles.error, {
    narrowedBy: `${project}|${group}|${unusedOnly}|${needle}`,
  });
  // Runs: under Project, a project's roles gather under its name (RULES: sort lives on the headings).
  // The count is the run's whole size, drawn from every row still in place.
  const runAt = useMemo(
    () =>
      runStarts(
        leaving.rows,
        view.sort.key === "project" ? (role) => projectLabel(role.project_name) : null,
      ),
    [leaving.rows, view.sort.key],
  );
  const unusedCount = all.filter((role) => role.is_unused).length;
  const filtered = Boolean(project || group || unusedOnly || needle);

  // One basis for the whole page — every row carries an identical copy, so
  // the freshness is stated here once rather than once per row.
  const basis = all[0]?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  function clearFilters() {
    setProject("");
    setGroup("");
    setQuery("");
    if (unusedOnly) router.replace("/roles");
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Roles"
        lede="Every role in every project, one named kind of access each. Open a role to see who holds it."
        actions={
          <Button variant="accent" onClick={() => setCreating(true)}>
            New role
          </Button>
        }
      />

      {/* Three selects in the page header stretched to the width of the
          actions area. They narrow the list rather than acting on it, so they
          belong on the list's own row — "New role" is the action, and stays. */}
      <FilterBar
        search={
          <Input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search role, key or project…"
            aria-label="Search roles by name, key or project"
          />
        }
        filters={
          <FilterPanel
            resultCount={rows.length}
            noun={["role", "roles"]}
            activeCount={[project, group, unusedOnly ? "1" : ""].filter(Boolean).length}
            onClear={() => {
              setProject("");
              setGroup("");
              router.replace("/roles");
            }}
          >
            <FilterField label="Project">
              <Select
                value={project}
                onChange={(event) => setProject(event.target.value)}
                aria-label="Filter by project"
              >
                <option value="">All projects</option>
                {projects.map((entry) => (
                  <option key={entry.id} value={entry.id}>
                    {entry.label}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="Group">
              <Select
                value={group}
                onChange={(event) => setGroup(event.target.value)}
                aria-label="Filter by group"
              >
                <option value="">All groups</option>
                {groups.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="Usage">
              <Select
                value={unusedOnly ? "unused" : ""}
                onChange={(event) =>
                  router.replace(event.target.value === "unused" ? "/roles?unused=1" : "/roles")
                }
                aria-label="Filter by usage"
              >
                <option value="">All roles</option>
                <option value="unused">Unused only ({unusedCount})</option>
              </Select>
            </FilterField>
          </FilterPanel>
        }
        /* The read line, and under it the one thing a reader must know about the list: it is not
           exhaustive. A caption, not a box (A-04 §7.1a). A role made straight in Zitadel on a project
           Syndra cannot read is missing, and a silently partial list is how somebody concludes a role
           doesn't exist and creates a duplicate. People is what Syndra decided, not what Zitadel has
           confirmed, so the freshness is stated once for the whole page. */
        read={
          all.length > 0 ? (
            <div className="flex flex-col gap-1">
              <ReadFreshness state={readState} subject="What Zitadel confirmed" />
              <p data-prose className="measure text-small leading-[1.55] text-muted [text-wrap:pretty]">
                Roles made straight in Zitadel, on a project Syndra cannot read, are not listed.{" "}
                <ArrowLink href="/zitadel/projects" className="whitespace-nowrap">
                  Check a project&rsquo;s roles in Zitadel
                </ArrowLink>
              </p>
            </div>
          ) : undefined
        }
      />

      <Card stale={readState.current === false}>
        <div className={view.className}>
        <ListStates
          head={
            <CardColumns cols={ROLE_COLS}>
              <SortHead label="Project" name="project" sort={sort} onSort={setSort} />
              <SortHead label="Role" name="role" sort={sort} onSort={setSort} />
              <SortHead label="Group" name="group" sort={sort} onSort={setSort} />
              <span>Used by</span>
              <SortHead label="People" name="people" sort={sort} onSort={setSort} />
            </CardColumns>
          }
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={leaving.rows.length === 0}
          onRetry={() => roles.refetch()}
          errorTitle="Couldn't load the role index."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading roles" />}
          empty={
            filtered ? (
              <EmptyState
                title={
                  unusedOnly
                    ? "Every role is referenced by something."
                    : needle && !project && !group
                      ? "No roles match that."
                      : "No roles match those filters."
                }
                action={{ label: "Clear filters", onClick: clearFilters }}
                resolved={unusedOnly}
              />
            ) : (
              <EmptyState title="No roles yet." />
            )
          }
        >
          {leaving.rows.map((role, index) => {
            const run = runAt(index);
            const underRun =
              view.sort.key === "project" && Boolean(role.project_name);
            const holders = holdersLine(
              role.assigned_user_count,
              role.confirmed_user_count,
              role.observed_user_count,
            );
            return (
            <LeavingRow
              key={`${role.project_id}:${role.role_key}`}
              {...leaving.props(`${role.project_id}:${role.role_key}`)}
            >
            {run && <RunLabel {...run} />}
            <Link
              data-row
              href={`/projects/${role.project_id}/roles/${encodeURIComponent(role.role_key)}`}
              className={`row-divider flex flex-col items-start gap-y-1 ${ROW} hover-tint tablet:flex-row tablet:flex-wrap tablet:items-center`}
            >
              <Cells cols={ROLE_COLS}>
              {/* Under the project's own label the name is not said again, unless Syndra could not
                  name it: then the id beside "Unknown project" is all there is to tell two apart. */}
              {underRun ? (
                <span aria-hidden />
              ) : (
                <span className="block text-fine text-muted [text-wrap:pretty] tablet:text-body">
                  <ProjectName id={role.project_id} name={role.project_name} link={false} />
                </span>
              )}
              <span>
                <span className="block text-body font-semibold [text-wrap:pretty]">
                  {roleName(role.display_name, role.role_key)}
                  {role.waits_for_send && <WaitsMark className="ml-2" />}{" "}
                  <Mono className="font-normal text-faint">{role.role_key}</Mono>
                </span>
                {role.cloned_from_role && (
                  <span className="block text-fine text-faint [text-wrap:pretty]">
                    cloned from <ProjectName id={role.cloned_from_project} link={false} /> / {role.cloned_from_role}
                  </span>
                )}
                {/* Between tablet and desktop the group and what uses the role
                    fold into a line under it, so the role keeps its room. */}
                <span className="block text-fine text-faint [text-wrap:pretty] desktop:hidden">
                  {[role.group, role.is_unused ? "Unused" : usedBy(role)]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
              <span className="text-small text-muted [text-wrap:pretty]">
                {role.group || "—"}
              </span>
              {/*
                What would break if this role went away. A role nobody holds may
                still be the input to a mapping rule or a member of a bundle,
                and "0 people" alone reads as safe to delete when it isn't.
              */}
              <span className="text-small text-muted [text-wrap:pretty]">
                {role.is_unused ? (
                  <Badge tone="neutral">Unused</Badge>
                ) : (
                  usedBy(role) || <span className="text-faint">—</span>
                )}
              </span>
              <span data-figure className="text-body">
                {holders.headline}
                <span className="text-small text-faint tablet:hidden">
                  {peopleSuffix(holders.headline)}
                </span>
                {/* Recorded is what Syndra decided; this is what Zitadel shows
                    right now — never the same word, never the same line. */}
                {holders.note && (
                  <span className={`block text-fine ${holdersToneClass[holders.tone]}`}>
                    {holders.note}
                  </span>
                )}
              </span>
              </Cells>
            </Link>
            </LeavingRow>
            );
          })}
        </ListStates>
        </div>
      </Card>

      {creating && <CreateRoleDialog onClose={() => setCreating(false)} />}
    </div>
  );
}

/** "2 bundles · 1 rule", and nothing at all when neither references it. */
function usedBy(role: CatalogRole): string {
  const parts: string[] = [];
  if (role.bundle_count) {
    parts.push(`${role.bundle_count} ${role.bundle_count === 1 ? "bundle" : "bundles"}`);
  }
  if (role.rule_count) {
    parts.push(`${role.rule_count} ${role.rule_count === 1 ? "rule" : "rules"}`);
  }
  return parts.join(" · ");
}

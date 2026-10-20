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
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterPanel, FilterField } from "@/components/ui/FilterPanel";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { Select } from "@/components/ui/Select";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { humanizeKey } from "@/lib/format";

type RoleSort = "project" | "role" | "group" | "people";

const ROLE_SORT: Record<RoleSort, (role: CatalogRole) => string | number> = {
  project: (role) => projectLabel(role.project_name, role.project_id),
  role: (role) => role.display_name || humanizeKey(role.role_key),
  group: (role) => role.group || "",
  // The number the row shows: what Zitadel holds, else what Syndra recorded.
  people: (role) => role.observed_user_count ?? role.assigned_user_count,
};

/**
 * E2 index · /roles — the cross-project role index, and the landing target for
 * the rail's Roles item.
 *
 * Project is the FIRST column and never collapses: the same key in two projects
 * means two different things, and a list that leads with the key invites
 * treating them as one role.
 */
export default function RolesPage() {
  const roles = useGlobalRoleCatalog();
  const router = useRouter();
  const params = useSearchParams();
  const [project, setProject] = useState("");
  const [group, setGroup] = useState("");
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [sort, setSort] = useSort<RoleSort>("project");

  // Unused lives in the URL, unlike the other two, because Home links straight
  // here with "N roles nobody holds" — a link that until now landed on an
  // unfiltered index and left the reader to find them.
  const unusedOnly = params.get("unused") === "1";

  const all = useMemo(() => roles.data ?? [], [roles.data]);

  // Keyed by id, labelled by name: two projects Syndra could not name are two
  // projects, not one "Unknown project", and an id is never the label.
  const projects = useMemo(() => {
    const byId = new Map(all.map((role) => [role.project_id, projectLabel(role.project_name, role.project_id)]));
    return Array.from(byId, ([id, label]) => ({ id, label })).sort((a, b) => a.label.localeCompare(b.label));
  }, [all]);
  const groups = useMemo(
    () => Array.from(new Set(all.map((role) => role.group).filter(Boolean))).sort() as string[],
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
    sort,
    ROLE_SORT,
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
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Roles"
        lede="Every role in every project — one named kind of access each. Open a role to see who holds it."
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
      />

      {/*
        The read line, and under it the one thing a reader must know about the
        list: it is not exhaustive. A caption, not a box (A-04 §7.1a; board,
        Cohesion choice 7). GET /api/v1/roles returns roles Syndra created plus
        whatever the directory reports; a role made directly upstream in a
        project Syndra cannot currently read is still missing, and a silently
        partial list is how somebody concludes a role doesn't exist and creates
        a duplicate. People is what Syndra decided, not what Zitadel has
        confirmed, so the freshness is stated once for the whole page.
      */}
      {all.length > 0 && (
        <div className="-mt-1.5 flex flex-col gap-1 px-[2px]">
          <ReadFreshness state={readState} subject="What Zitadel confirmed" />
          <p className="max-w-[92ch] text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
            Roles made straight in Zitadel, on a project Syndra cannot read, are not listed.{" "}
            <ArrowLink href="/zitadel/projects" className="whitespace-nowrap">
              Check a project&rsquo;s roles in Zitadel
            </ArrowLink>
          </p>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[180px] shrink-0">
            <SortHead label="Project" name="project" sort={sort} onSort={setSort} />
          </span>
          <span className="min-w-[200px] flex-1">
            <SortHead label="Role" name="role" sort={sort} onSort={setSort} />
          </span>
          <span className="hidden w-[130px] shrink-0 desktop:block">
            <SortHead label="Group" name="group" sort={sort} onSort={setSort} />
          </span>
          <span className="hidden w-[150px] shrink-0 desktop:block">Used by</span>
          <span className="w-[110px] shrink-0 text-right">
            <SortHead label="People" name="people" sort={sort} onSort={setSort} />
          </span>
        </CardColumns>

        <ListStates
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={rows.length === 0}
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
                guidance="Clear a filter, or check Zitadel for roles Syndra didn't create."
                action={{ label: "Clear filters", onClick: clearFilters }}
              />
            ) : (
              <EmptyState
                title="No roles yet."
                guidance="Create one here, or check Zitadel for roles Syndra didn't create."
                action={{ label: "Create a role", onClick: () => setCreating(true) }}
              />
            )
          }
        >
          {rows.map((role) => {
            const holders = holdersLine(
              role.assigned_user_count,
              role.confirmed_user_count,
              role.observed_user_count,
            );
            return (
            <Link
              key={`${role.project_id}:${role.role_key}`}
              href={`/projects/${role.project_id}/roles/${encodeURIComponent(role.role_key)}`}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1 px-5 py-3 motion-tint hover:bg-[var(--hover)] tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              <span className="w-full text-[12.5px] text-muted [text-wrap:pretty] tablet:w-[180px] tablet:shrink-0 tablet:text-[14.5px]">
                <ProjectName id={role.project_id} name={role.project_name} />
              </span>
              <span className="w-full min-w-0 tablet:min-w-[200px] tablet:flex-1">
                <span className="block text-[15px] font-semibold [text-wrap:pretty]">
                  {role.display_name || humanizeKey(role.role_key)}
                  {role.waits_for_send && <WaitsMark className="ml-2" />}{" "}
                  <Mono className="font-normal text-faint">{role.role_key}</Mono>
                </span>
                {role.cloned_from_role && (
                  <span className="block text-[12.5px] text-faint [text-wrap:pretty]">
                    cloned from <ProjectName id={role.cloned_from_project} /> / {role.cloned_from_role}
                  </span>
                )}
                {/* Between tablet and desktop the group and what uses the role
                    fold into a line under it, so the role keeps its room. */}
                <span className="block text-[12.5px] text-faint [text-wrap:pretty] desktop:hidden">
                  {[role.group, role.is_unused ? "Used by nothing" : usedBy(role)]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
              <span className="hidden w-[130px] shrink-0 text-[13.5px] text-muted [text-wrap:pretty] desktop:block">
                {role.group || "—"}
              </span>
              {/*
                What would break if this role went away. A role nobody holds may
                still be the input to a mapping rule or a member of a bundle,
                and "0 members" alone reads as safe to delete when it isn't.
              */}
              <span className="hidden w-[150px] shrink-0 text-[13.5px] text-muted [text-wrap:pretty] desktop:block">
                {role.is_unused ? (
                  <Badge tone="neutral">Unused</Badge>
                ) : (
                  usedBy(role) || <span className="text-faint">—</span>
                )}
              </span>
              <span className="w-full text-[15px] tablet:w-[110px] tablet:shrink-0 tablet:text-right">
                {holders.headline}
                <span className="text-[13px] text-faint tablet:hidden">
                  {holders.headline === "1" ? " person" : " people"}
                </span>
                {/* Recorded is what Syndra decided; this is what Zitadel shows
                    right now — never the same word, never the same line. */}
                {holders.note && (
                  <span className={`block text-[12.5px] ${holdersToneClass[holders.tone]}`}>
                    {holders.note}
                  </span>
                )}
              </span>
            </Link>
            );
          })}
        </ListStates>
      </Card>

      <p className="max-w-[900px] text-[14px] leading-[1.55] text-faint">
        Same key, two projects, two different things — which is why the project column is first and
        never collapses.
      </p>

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

"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { CreateRoleDialog } from "@/components/roles/CreateRoleDialog";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

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

  // Unused lives in the URL, unlike the other two, because Today links straight
  // here with "N roles nobody holds" — a link that until now landed on an
  // unfiltered index and left the reader to find them.
  const unusedOnly = params.get("unused") === "1";

  const all = useMemo(() => roles.data ?? [], [roles.data]);

  const projects = useMemo(
    () => Array.from(new Set(all.map((role) => role.project_name || role.project_id))).sort(),
    [all],
  );
  const groups = useMemo(
    () => Array.from(new Set(all.map((role) => role.group).filter(Boolean))).sort() as string[],
    [all],
  );

  const rows = all.filter(
    (role) =>
      (!project || (role.project_name || role.project_id) === project) &&
      (!group || role.group === group) &&
      (!unusedOnly || role.is_unused),
  );
  const unusedCount = all.filter((role) => role.is_unused).length;
  const filtered = Boolean(project || group || unusedOnly);

  function clearFilters() {
    setProject("");
    setGroup("");
    if (unusedOnly) router.replace("/roles");
  }

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
              className="w-[180px]"
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
            <Select
              value={unusedOnly ? "unused" : ""}
              onChange={(event) =>
                router.replace(event.target.value === "unused" ? "/roles?unused=1" : "/roles")
              }
              aria-label="Filter by usage"
              className="w-[190px]"
            >
              <option value="">All roles</option>
              <option value="unused">Unused only ({unusedCount})</option>
            </Select>
            <Button variant="accent" onClick={() => setCreating(true)}>
              New role
            </Button>
          </>
        }
      />

      {/*
        Required scope notice. GET /api/v1/roles returns roles Syndra created
        plus whatever the directory reports; a role made directly upstream in a
        project Syndra cannot currently read is still missing. Stating that is
        not pedantry — a silently partial list is how somebody concludes a role
        doesn't exist and creates a duplicate.
      */}
      <div className="accent-note flex items-start gap-3 px-[18px] py-3.5">
        <span
          aria-hidden
          className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-accent-soft text-[12px] font-bold text-accent-text"
        >
          i
        </span>
        <div className="text-[14px] leading-[1.55] text-ink/[.78]">
          <strong className="font-semibold text-ink">This list may be partial.</strong> It covers
          roles Syndra created and roles the directory reports; anything created directly in the
          identity provider on a project Syndra cannot read is not here.{" "}
          <Link href="/zitadel/projects" className="font-semibold text-accent-text">
            Check a project&rsquo;s roles upstream →
          </Link>
        </div>
      </div>

      <Card>
        <CardColumns>
          <span className="w-[180px]">Project</span>
          <span className="flex-1">Role</span>
          <span className="w-[130px]">Group</span>
          <span className="w-[150px]">Used by</span>
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
            filtered ? (
              <EmptyState
                title={
                  unusedOnly
                    ? "Every role is referenced by something."
                    : "No roles match those filters."
                }
                guidance="Clear a filter, or check the identity provider for roles Syndra didn't create."
                action={{ label: "Clear filters", onClick: clearFilters }}
              />
            ) : (
              <EmptyState
                title="No roles yet."
                guidance="Create one here, or check the identity provider for roles Syndra didn't create."
                action={{ label: "Create a role", onClick: () => setCreating(true) }}
              />
            )
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
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[15px] font-semibold">
                  {role.display_name || humanizeKey(role.role_key)}{" "}
                  <Mono className="font-normal text-faint">{role.role_key}</Mono>
                </span>
                {role.cloned_from_role && (
                  <span className="block truncate text-[12.5px] text-faint">
                    cloned from {role.cloned_from_project} / {role.cloned_from_role}
                  </span>
                )}
              </span>
              <span className="w-[130px] truncate text-[13.5px] text-muted">
                {role.group || "—"}
              </span>
              {/*
                What would break if this role went away. A role nobody holds may
                still be the input to a mapping rule or a member of a bundle,
                and "0 members" alone reads as safe to delete when it isn't.
              */}
              <span className="w-[150px] truncate text-[13.5px] text-muted">
                {role.is_unused ? (
                  <Badge tone="neutral">Unused</Badge>
                ) : (
                  usedBy(role) || <span className="text-faint">—</span>
                )}
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

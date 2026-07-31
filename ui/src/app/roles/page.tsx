"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useProjects } from "@/lib/queries/useProjects";
import { useCreateRole, useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
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
  const [project, setProject] = useState("");
  const [group, setGroup] = useState("");
  const [creating, setCreating] = useState(false);

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
      (!group || role.group === group),
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
            <Button variant="accent" onClick={() => setCreating(true)}>
              New role
            </Button>
          </>
        }
      />

      {/*
        Required scope notice. GET /api/v1/roles returns roles MkAuth created
        plus whatever the directory reports; a role made directly upstream in a
        project MkAuth cannot currently read is still missing. Stating that is
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
          roles MkAuth created and roles the directory reports; anything created directly in the
          identity provider on a project MkAuth cannot read is not here.{" "}
          <Link href="/zitadel/projects" className="font-semibold text-accent-text">
            Check a project&rsquo;s roles upstream →
          </Link>
        </div>
      </div>

      <Card>
        <CardColumns>
          <span className="w-[180px]">Project</span>
          <span className="flex-1">Role</span>
          <span className="w-[150px]">Group</span>
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
              action={{
                label: "Clear filters",
                onClick: () => {
                  setProject("");
                  setGroup("");
                },
              }}
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
              <span className="w-[150px] truncate text-[13.5px] text-muted">
                {role.group || "—"}
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

      {creating && <CreateRoleDialog catalog={all} onClose={() => setCreating(false)} />}
    </div>
  );
}

/**
 * Creating a role through MkAuth writes it locally AND upstream in one action,
 * rolling the local row back if the identity provider refuses. That is the
 * difference between this and creating one directly in the provider, where
 * MkAuth learns about it only when the drift sweep flags it as unexplained.
 *
 * Clone-from copies the display name and description of an existing role, and
 * records the provenance — "cloned from Metal Shop / trained" is what tells a
 * later reader the two are deliberately related.
 */
function CreateRoleDialog({
  catalog,
  onClose,
}: {
  catalog: CatalogRole[];
  onClose: () => void;
}) {
  const projects = useProjects();
  const create = useCreateRole();

  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [description, setDescription] = useState("");
  const [group, setGroup] = useState("");
  const [cloneFrom, setCloneFrom] = useState("");

  const valid = /^[a-zA-Z0-9_-]+$/.test(roleKey);
  const duplicate = catalog.some(
    (role) => role.project_id === projectId && role.role_key === roleKey,
  );

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="md" labelledBy="new-role-title">
      <ModalHeader
        title="New role"
        titleId="new-role-title"
        lede="Created in MkAuth and in the identity provider together — if the provider refuses, nothing is left behind here."
      />

      <div className="flex flex-col gap-3.5 px-6">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="role-project">Project</FieldLabel>
            <Select
              id="role-project"
              value={projectId}
              onChange={(event) => setProjectId(event.target.value)}
            >
              <option value="">Choose…</option>
              {(projects.data ?? []).map((entry) => (
                <option key={entry.project.id} value={entry.project.id}>
                  {entry.project.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <FieldLabel htmlFor="role-key">Role key</FieldLabel>
            <Input
              id="role-key"
              value={roleKey}
              onChange={(event) => setRoleKey(event.target.value)}
              placeholder="trained"
            />
            <FieldHint>
              {roleKey && !valid
                ? "Letters, numbers, dashes and underscores only."
                : duplicate
                  ? "That key already exists in this project."
                  : "This is what appears in a token. It cannot be changed later."}
            </FieldHint>
          </div>
        </div>

        <div>
          <FieldLabel htmlFor="role-name">Display name</FieldLabel>
          <Input
            id="role-name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Trained operator"
          />
        </div>

        <div>
          <FieldLabel htmlFor="role-description">What can somebody with it do?</FieldLabel>
          <Input
            id="role-description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Can cut and engrave unsupervised."
          />
          <FieldHint>
            Shown in full wherever this role is listed. &ldquo;Can cut unsupervised&rdquo; versus
            &ldquo;may enter and watch&rdquo; is the entire decision an operator makes.
          </FieldHint>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="role-group">Group</FieldLabel>
            <Input
              id="role-group"
              value={group}
              onChange={(event) => setGroup(event.target.value)}
              placeholder="Safety-gated"
            />
          </div>
          <div>
            <FieldLabel htmlFor="role-clone">Clone from</FieldLabel>
            <Select
              id="role-clone"
              value={cloneFrom}
              onChange={(event) => setCloneFrom(event.target.value)}
            >
              <option value="">Nothing — start empty</option>
              {catalog.map((role) => (
                <option
                  key={`${role.project_id}:${role.role_key}`}
                  value={`${role.project_id}:${role.role_key}`}
                >
                  {role.project_name} / {role.role_key}
                </option>
              ))}
            </Select>
            <FieldHint>Copies the name and description, and records where it came from.</FieldHint>
          </div>
        </div>
      </div>

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!projectId || !roleKey || !valid || duplicate}
          isPending={create.isPending}
          onClick={async () => {
            const [cloneProject, cloneRole] = cloneFrom.split(":");
            try {
              await create.mutateAsync({
                project_id: projectId,
                role_key: roleKey,
                display_name: displayName,
                description,
                group,
                clone_from: cloneFrom
                  ? { project_id: cloneProject, role_key: cloneRole }
                  : undefined,
              });
              toast.success(`${roleKey} created. Nobody holds it yet.`);
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The role wasn't created.");
            }
          }}
        >
          Create role
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

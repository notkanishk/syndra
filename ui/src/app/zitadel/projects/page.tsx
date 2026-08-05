"use client";

import { useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DirectWriteWarning, UpstreamShell } from "@/components/upstream/UpstreamShell";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import {
  useUpstreamCreateRole,
  useUpstreamDeleteRole,
  useUpstreamProjectRoles,
  useUpstreamProjects,
  useUpstreamUpdateRole,
  type UpstreamProjectRole,
} from "@/lib/queries/useUpstream";

/**
 * Projects and their roles, as the identity provider holds them.
 *
 * This is the list `/roles` cannot show: GET /api/v1/roles returns only roles
 * Syndra created, so a role somebody made upstream is invisible there. That is
 * exactly the gap this page exists to close.
 */
export default function UpstreamProjectsPage() {
  const projects = useUpstreamProjects();
  const [selected, setSelected] = useState<string | null>(null);
  const [editing, setEditing] = useState<UpstreamProjectRole | "new" | null>(null);

  const rows = projects.data?.items ?? [];
  const activeId = selected ?? rows[0]?.id ?? null;
  const roles = useUpstreamProjectRoles(activeId);

  return (
    <UpstreamShell
      title="Projects and their roles"
      lede="Read live from the identity provider, including roles Syndra never created."
      syndraHref="/projects"
      syndraLabel="See the same projects as Syndra understands them"
    >
      <div className="flex flex-wrap items-start gap-5">
        <Card className="w-[280px] min-w-[240px] flex-none">
          <CardHeader title="Projects" count={rows.length} />
          <ListStates
            isLoading={projects.isLoading}
            error={projects.error}
            isEmpty={rows.length === 0}
            onRetry={() => projects.refetch()}
            errorTitle="Couldn't read projects from the identity provider."
            skeleton={<RowSkeleton rows={5} avatar={false} label="Reading projects" />}
            empty={
              <EmptyState
                title="No projects upstream."
                guidance="Either none exist, or the service account cannot see them."
              />
            }
          >
            {rows.map((project) => (
              <button
                key={project.id}
                type="button"
                onClick={() => setSelected(project.id)}
                aria-current={activeId === project.id ? "true" : undefined}
                className={`row-divider flex w-full flex-col items-start px-4 py-3 text-left motion-tint ${
                  activeId === project.id ? "bg-accent-soft/60" : "hover:bg-[var(--hover)]"
                }`}
              >
                <span className="truncate text-[14.5px] font-semibold">{project.name}</span>
                <Mono className="truncate text-faint">{project.id}</Mono>
              </button>
            ))}
          </ListStates>
        </Card>

        <Card className="min-w-[420px] flex-1">
          <CardHeader
            title="Roles in this project"
            count={roles.data?.items.length}
            action={
              activeId ? (
                <Button size="sm" variant="danger" onClick={() => setEditing("new")}>
                  New role upstream
                </Button>
              ) : undefined
            }
          />

          <CardColumns>
            <span className="flex-1">Role</span>
            <span className="w-[160px]">Group</span>
            <span className="w-[160px] text-right">Change</span>
          </CardColumns>

          <ListStates
            isLoading={roles.isLoading}
            error={roles.error}
            isEmpty={(roles.data?.items ?? []).length === 0}
            onRetry={() => roles.refetch()}
            errorTitle="Couldn't read this project's roles."
            skeleton={<RowSkeleton rows={4} avatar={false} label="Reading roles" />}
            empty={
              <EmptyState
                title="This project has no roles upstream."
                guidance="Create one here, or — preferably — create it in Syndra so it is tracked."
              />
            }
          >
            {(roles.data?.items ?? []).map((role) => (
              <RoleRow
                key={role.key}
                projectId={activeId!}
                role={role}
                onEdit={() => setEditing(role)}
              />
            ))}
          </ListStates>
        </Card>
      </div>

      {editing && activeId && (
        <RoleDialog
          projectId={activeId}
          role={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}
    </UpstreamShell>
  );
}

function RoleRow({
  projectId,
  role,
  onEdit,
}: {
  projectId: string;
  role: UpstreamProjectRole;
  onEdit: () => void;
}) {
  const remove = useUpstreamDeleteRole();
  const [confirming, setConfirming] = useState(false);

  return (
    <>
      <div className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
        <span className="min-w-[200px] flex-1 truncate text-[14.5px]">
          {role.displayName || role.key} <Mono className="text-faint">{role.key}</Mono>
        </span>
        <span className="w-[160px] truncate text-[13.5px] text-muted">{role.group || "—"}</span>
        <span className="flex w-[160px] justify-end gap-2">
          <Button size="sm" onClick={onEdit}>
            Edit
          </Button>
          <Button size="sm" variant="danger" onClick={() => setConfirming(true)}>
            Delete
          </Button>
        </span>
      </div>

      {confirming && (
        <Modal open onClose={() => setConfirming(false)} busy={remove.isPending} size="sm">
          <ModalHeader
            title={`Delete ${role.displayName || role.key} upstream?`}
            lede="The role disappears from the identity provider, and everybody currently holding it loses it."
          />
          <div className="px-6">
            <DirectWriteWarning what="Deleting a role removes it for every holder at once." />
          </div>
          <ModalFooter>
            <Button
              variant="dangerConfirm"
              isPending={remove.isPending}
              onClick={async () => {
                try {
                  await remove.mutateAsync({ projectId, key: role.key });
                  toast.success(`${role.key} deleted upstream.`);
                  setConfirming(false);
                } catch (error) {
                  toast.error(error instanceof Error ? error.message : "That didn't go through.");
                }
              }}
            >
              Delete role
            </Button>
            <Button onClick={() => setConfirming(false)}>Cancel</Button>
          </ModalFooter>
        </Modal>
      )}
    </>
  );
}

function RoleDialog({
  projectId,
  role,
  onClose,
}: {
  projectId: string;
  role: UpstreamProjectRole | null;
  onClose: () => void;
}) {
  const create = useUpstreamCreateRole();
  const update = useUpstreamUpdateRole();
  const [key, setKey] = useState(role?.key ?? "");
  const [displayName, setDisplayName] = useState(role?.displayName ?? "");
  const [group, setGroup] = useState(role?.group ?? "");

  const busy = create.isPending || update.isPending;

  return (
    <Modal open onClose={onClose} busy={busy} size="sm">
      <ModalHeader
        title={role ? `Edit ${role.key} upstream` : "New role upstream"}
        lede="Roles created here are not tracked by Syndra until somebody adopts them."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="upstream-role-key">Role key</FieldLabel>
          <Input
            id="upstream-role-key"
            value={key}
            disabled={Boolean(role)}
            onChange={(event) => setKey(event.target.value)}
            placeholder="trained"
          />
        </div>
        <div>
          <FieldLabel htmlFor="upstream-role-name">Display name</FieldLabel>
          <Input
            id="upstream-role-name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Trained operator"
          />
        </div>
        <div>
          <FieldLabel htmlFor="upstream-role-group">Group</FieldLabel>
          <Input
            id="upstream-role-group"
            value={group}
            onChange={(event) => setGroup(event.target.value)}
            placeholder="Safety-gated"
          />
        </div>
        <DirectWriteWarning what="Creating or renaming a role here happens outside Syndra's record." />
      </div>
      <ModalFooter>
        <Button
          variant="danger"
          disabled={!key.trim()}
          isPending={busy}
          onClick={async () => {
            try {
              if (role) {
                await update.mutateAsync({ projectId, key: role.key, displayName, group });
                toast.success(`${role.key} updated upstream.`);
              } else {
                await create.mutateAsync({ projectId, key: key.trim(), displayName, group });
                toast.success(`${key.trim()} created upstream.`);
              }
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "That didn't go through.");
            }
          }}
        >
          {role ? "Save upstream" : "Create upstream"}
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

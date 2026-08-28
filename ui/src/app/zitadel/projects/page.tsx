"use client";

import { useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DirectWriteWarning, UpstreamShell } from "@/components/upstream/UpstreamShell";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import {
  useUpstreamCreateRole,
  useUpstreamDeleteRole,
  useUpstreamProjectRoles,
  useUpstreamProjects,
  useUpstreamUpdateRole,
  type UpstreamProjectRole,
} from "@/lib/queries/useUpstream";

/** What is recorded when a change goes straight to Zitadel: one line, and no explanation. */
const AUDIT_ONLY =
  "Syndra recorded one line in Audit and nothing else; it will not explain this change anywhere.";

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
      lede="Read live from Zitadel, including roles Syndra never created."
      syndraHref="/projects"
      syndraLabel="See the same projects as Syndra understands them"
    >
      <div className="flex flex-col items-stretch gap-5 tablet:flex-row tablet:flex-wrap tablet:items-start">
        <Card className="w-[280px] min-w-[240px] flex-none">
          <CardHeader title="Projects" count={rows.length} />
          <ListStates
            isLoading={projects.isLoading}
            error={projects.error}
            isEmpty={rows.length === 0}
            onRetry={() => projects.refetch()}
            errorTitle="Couldn't read projects from Zitadel. Syndra itself is fine."
            skeleton={<RowSkeleton rows={5} avatar={false} label="Reading projects" />}
            empty={
              <EmptyState
                title="Zitadel has no projects."
                guidance="Either there are none, or the account Syndra uses to read Zitadel is not allowed to see them."
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

        <Card className="w-full tablet:min-w-[420px] tablet:flex-1">
          <CardHeader
            title="Roles in this project"
            count={roles.data?.items.length}
            action={
              activeId ? (
                <Button size="sm" variant="danger" onClick={() => setEditing("new")}>
                  New role in Zitadel
                </Button>
              ) : undefined
            }
          />

          <CardColumns>
            <span className="flex-1">Role</span>
            <span className="w-[160px]">Group</span>
            <span className="w-[160px]" />
          </CardColumns>

          <ListStates
            isLoading={roles.isLoading}
            error={roles.error}
            isEmpty={(roles.data?.items ?? []).length === 0}
            onRetry={() => roles.refetch()}
            errorTitle="Couldn't read this project's roles from Zitadel. Syndra itself is fine."
            skeleton={<RowSkeleton rows={4} avatar={false} label="Reading roles" />}
            empty={
              <EmptyState
                title="This project has no roles in Zitadel."
                guidance="Create one here, or — better — create it in Syndra under Access › Roles, so Syndra keeps track of it."
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [acknowledged, setAcknowledged] = useState(false);

  return (
    <>
      <div className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4">
        <span className="min-w-[200px] flex-1 truncate text-[14.5px]">
          {role.displayName || role.key} <Mono className="text-faint">{role.key}</Mono>
        </span>
        <span className="w-[160px] truncate text-[13.5px] text-muted">{role.group || "—"}</span>
        <span className="flex w-[160px] justify-end gap-2">
          <Button size="sm" onClick={onEdit} aria-label={`Edit ${role.displayName || role.key}`}>
            Edit
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setConfirming(true)}
            aria-label={`Delete ${role.displayName || role.key}`}
          >
            Delete
          </Button>
        </span>
      </div>

      {confirming && (
        <Modal open onClose={() => setConfirming(false)} busy={remove.isPending} size="sm">
          <ModalHeader
            title={`Delete ${role.displayName || role.key} in Zitadel?`}
            lede="The role disappears from Zitadel, and everybody who holds it loses it at once."
          />
          <div className="px-6">
            <DirectWriteWarning
              what="Deleting a role revokes it (ends access) for everyone who holds it, the moment you press the button."
              acknowledged={acknowledged}
              onAcknowledge={setAcknowledged}
            />
          </div>
          {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
            <Button
              variant="dangerConfirm"
              disabled={!acknowledged}
              isPending={remove.isPending}
              onClick={async () => {
                try {
                  await remove.mutateAsync({ projectId, key: role.key });
                  // "Applied", and it means it here in a way it does not
                  // anywhere else in the product: this write went straight to
                  // Zitadel with no plan, no queue and no ledger row behind it.
                  setOutcome({
                    kind: "applied",
                    message: `${role.key} deleted in Zitadel`,
                    detail: AUDIT_ONLY,
                  });
                } catch (error) {
                  setOutcome(outcomeFromError(error));
                }
              }}
            >
              Delete role in Zitadel
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const update = useUpstreamUpdateRole();
  const [key, setKey] = useState(role?.key ?? "");
  const [displayName, setDisplayName] = useState(role?.displayName ?? "");
  const [group, setGroup] = useState(role?.group ?? "");
  const [acknowledged, setAcknowledged] = useState(false);

  const busy = create.isPending || update.isPending;

  return (
    <Modal open onClose={onClose} busy={busy} size="sm">
      <ModalHeader
        title={role ? `Edit ${role.key} in Zitadel` : "New role in Zitadel"}
        lede="Syndra will not know about this role until someone adds it under Access › Roles."
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
          <FieldHint>The short name other systems see, e.g. trained. Lowercase, no spaces.</FieldHint>
        </div>
        <div>
          <FieldLabel htmlFor="upstream-role-name">Display name</FieldLabel>
          <Input
            id="upstream-role-name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Trained on the laser cutter"
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
          <FieldHint>Optional. A label Zitadel uses to group related roles; Syndra ignores it.</FieldHint>
        </div>
        <DirectWriteWarning
          what="Creating or renaming a role here happens outside Syndra's record."
          acknowledged={acknowledged}
          onAcknowledge={setAcknowledged}
        />
      </div>
      {/* The dialog reports its own result and stays open to do it, the same
          way the role dialog on Syndra's own side does. Here it matters more:
          the sentence this reports is that Syndra has no record of what just
          happened, and a dialog that closes itself takes that sentence with
          it. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1 mt-3" />}

      <ModalFooter>
        <Button
          variant="danger"
          disabled={!key.trim() || !acknowledged || outcome?.kind === "applied"}
          isPending={busy}
          onClick={async () => {
            try {
              if (role) {
                await update.mutateAsync({ projectId, key: role.key, displayName, group });
                setOutcome({
                  kind: "applied",
                  message: `${role.key} updated in Zitadel`,
                  detail: AUDIT_ONLY,
                });
              } else {
                await create.mutateAsync({ projectId, key: key.trim(), displayName, group });
                setOutcome({
                  kind: "applied",
                  message: `${key.trim()} created in Zitadel`,
                  detail: AUDIT_ONLY,
                });
              }
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          {role ? "Save in Zitadel" : "Create in Zitadel"}
        </Button>
        <Button onClick={onClose}>{outcome?.kind === "applied" ? "Done" : "Cancel"}</Button>
      </ModalFooter>
    </Modal>
  );
}

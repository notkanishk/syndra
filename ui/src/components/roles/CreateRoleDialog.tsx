"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { roleLabel } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useProjects } from "@/lib/queries/useProjects";
import { useCreateRole, useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";

/**
 * Creating a role through Syndra writes it locally AND upstream in one action,
 * rolling the local row back if the identity provider refuses. That is the
 * difference between this and creating one directly in the provider, where
 * Syndra learns about it only when the drift sweep flags it as unexplained.
 *
 * Clone-from copies the display name and description of an existing role, and
 * records the provenance — "cloned from Metal Shop / trained" is what tells a
 * later reader the two are deliberately related.
 *
 * `pinnedProjectId` is what makes this reusable from a project's own page. The
 * most natural place to add a role to Printing Lab is the Printing Lab page,
 * and arriving there to be asked which project you meant is the kind of small
 * insult that sends people to the identity provider's console instead — where
 * the role is created with no local row and comes back as drift.
 */
export function CreateRoleDialog({
  pinnedProjectId,
  onClose,
}: {
  pinnedProjectId?: string;
  onClose: () => void;
}) {
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();
  const create = useCreateRole();
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const [projectId, setProjectId] = useState(pinnedProjectId ?? "");
  const [roleKey, setRoleKey] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [description, setDescription] = useState("");
  const [group, setGroup] = useState("");
  const [cloneFrom, setCloneFrom] = useState("");

  const all: CatalogRole[] = catalog.data ?? [];
  const pinnedName = projects.data?.find((entry) => entry.project.id === pinnedProjectId)?.project
    .name;

  const valid = /^[a-zA-Z0-9_-]+$/.test(roleKey);
  const duplicate = all.some((role) => role.project_id === projectId && role.role_key === roleKey);

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="md" labelledBy="new-role-title">
      <ModalHeader
        title="New role"
        titleId="new-role-title"
        lede="Created in Syndra and in Zitadel (the service everyone signs in through) at the same time. If Zitadel refuses it, nothing is created in either place."
      />

      <div className="flex flex-col gap-3.5 px-6">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="role-project">Project</FieldLabel>
            {pinnedProjectId ? (
              // Stated, not editable. The dialog was opened from this project's
              // own page; a select here would invite changing it by accident.
              <div className="flex items-center rounded-inner border border-line-strong px-[15px] py-3 text-[15px]">
                {pinnedName ?? pinnedProjectId}
              </div>
            ) : (
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
            )}
            <FieldHint>
              A machine or area with its own set of roles — the Laser Cutter, the Studio. Apps
              check a person&rsquo;s roles against one project.
            </FieldHint>
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
                  : "The short name other systems see for this role. It cannot be changed later."}
            </FieldHint>
          </div>
        </div>

        <div>
          <FieldLabel htmlFor="role-name">Display name</FieldLabel>
          <Input
            id="role-name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Laser trained"
          />
        </div>

        <div>
          <FieldLabel htmlFor="role-description">What can somebody with this role do?</FieldLabel>
          <Input
            id="role-description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Can cut and engrave unsupervised."
          />
          <FieldHint>
            Shown in full wherever this role is listed. &ldquo;Can cut unsupervised&rdquo; versus
            &ldquo;may enter and watch&rdquo; is the whole decision you make when you give it to someone.
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
            <FieldHint>Optional. Roles in the same group can be filtered together on the Roles page.</FieldHint>
          </div>
          <div>
            <FieldLabel htmlFor="role-clone">Clone from</FieldLabel>
            <Select
              id="role-clone"
              value={cloneFrom}
              onChange={(event) => setCloneFrom(event.target.value)}
            >
              <option value="">Nothing — start empty</option>
              {all.map((role) => (
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

      {/* The sheet reports its own result and stays open to do it: a role that
          nobody holds yet is a fact the operator has to act on next, and a
          dialog that closes itself takes that sentence with it. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!projectId || !roleKey || !valid || duplicate}
          isPending={create.isPending}
          onClick={async () => {
            const [cloneProject, cloneRole] = cloneFrom.split(":");
            const project =
              all.find((role) => role.project_id === projectId)?.project_name ??
              projects.data?.find((entry) => entry.project.id === projectId)?.project.name ??
              projectId;
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
              setOutcome({
                kind: "applied",
                message: `${roleLabel(project, roleKey, displayName)} created`,
                detail: "Nobody holds it yet. Give it to someone from People, or add it to a bundle.",
              });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Create role
        </Button>
        <Button onClick={onClose}>{outcome?.kind === "applied" ? "Done" : "Cancel"}</Button>
      </ModalFooter>
    </Modal>
  );
}

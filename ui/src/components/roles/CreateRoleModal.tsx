"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/Button";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { ApiError } from "@/lib/api-client";
import { useGlobalRoleCatalog, useCreateRole, type CatalogRole } from "@/lib/queries/useRoles";
import { toastError, toastSuccess } from "@/lib/toast";

interface CreateRoleModalProps {
  open: boolean;
  onClose: () => void;
  /**
   * Optional pre-fill — when set the project select is pinned to this id and
   * the picker is hidden. Used by deep-link flows like
   * `/bundles?createRole=p-1`.
   */
  initialProjectId?: string;
  /** Optional callback after a successful create. */
  onCreated?: (role: { project_id: string; role_key: string }) => void;
}

const ROLE_KEY_PATTERN = /^[a-z][a-z0-9_-]{0,63}$/;

/**
 * Slug-derive a candidate role_key from a display name. Operators retain
 * full override via the role_key field — derivation just gives them a
 * sensible starting point so they don't have to think about the slug shape.
 */
function deriveRoleKey(displayName: string): string {
  return displayName
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "") // strip diacritics
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 64);
}

/**
 * Role authoring modal — Stage 4 surfacing of the existing
 * `POST /api/v1/roles` endpoint. Supports clone-from for cases where the
 * operator wants to mirror an existing role's metadata into a new project.
 *
 * Slug derivation is one-way until the operator types in the role_key field;
 * after that the field is treated as locked so accidental edits to the
 * display name don't clobber an intentional override.
 *
 * Backend uniqueness errors (409 → ApiError code "CONFLICT") surface inline
 * as a field-level error so the operator can adjust without losing state;
 * other errors fall through to a Sonner toast.
 */
export default function CreateRoleModal({
  open,
  onClose,
  initialProjectId,
  onCreated,
}: CreateRoleModalProps) {
  const catalogQuery = useGlobalRoleCatalog();
  const createRole = useCreateRole();

  const [displayName, setDisplayName] = useState("");
  const [roleKey, setRoleKey] = useState("");
  const [keyTouched, setKeyTouched] = useState(false);
  const [projectId, setProjectId] = useState(initialProjectId ?? "");
  const [description, setDescription] = useState("");
  const [cloneFromKey, setCloneFromKey] = useState(""); // "<projectId>:<roleKey>" or ""
  const [conflictMsg, setConflictMsg] = useState<string | null>(null);

  // Reset every time the modal closes.
  useEffect(() => {
    if (!open) {
      setDisplayName("");
      setRoleKey("");
      setKeyTouched(false);
      setProjectId(initialProjectId ?? "");
      setDescription("");
      setCloneFromKey("");
      setConflictMsg(null);
    }
  }, [open, initialProjectId]);

  // Auto-derive role_key from display name until the operator manually edits.
  useEffect(() => {
    if (keyTouched) return;
    setRoleKey(deriveRoleKey(displayName));
  }, [displayName, keyTouched]);

  // Project list derived from the catalog — operators pick a target project
  // for the new role. Distinct project_id sorted by project name.
  const projects = useMemo(() => {
    const all = catalogQuery.data ?? [];
    const map = new Map<string, string>();
    for (const r of all) {
      if (!map.has(r.project_id)) map.set(r.project_id, r.project_name);
    }
    return Array.from(map.entries())
      .map(([id, name]) => ({ id, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [catalogQuery.data]);

  // Default the project select to the first known project once the catalog
  // resolves, unless the caller pinned a specific project.
  useEffect(() => {
    if (initialProjectId) return;
    if (projectId) return;
    if (projects.length === 0) return;
    setProjectId(projects[0].id);
  }, [projects, projectId, initialProjectId]);

  // Filter clone-from candidates to roles in the *currently selected* project
  // so the suggested clone source is something the new role would be a peer
  // of. The backend resolves clone metadata by (project_id, role_key) so any
  // project's role is technically valid; scoping the picker keeps the choice
  // tractable.
  const cloneCandidates: CatalogRole[] = useMemo(() => {
    const all = catalogQuery.data ?? [];
    if (!projectId) return [];
    return all
      .filter((r) => r.project_id === projectId)
      .sort((a, b) => a.display_name.localeCompare(b.display_name));
  }, [catalogQuery.data, projectId]);

  const slugValid = roleKey.length > 0 && ROLE_KEY_PATTERN.test(roleKey);
  const canSubmit =
    !!projectId && slugValid && (displayName.trim().length > 0 || cloneFromKey !== "");

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    setConflictMsg(null);

    let cloneFrom: { project_id: string; role_key: string } | undefined;
    if (cloneFromKey) {
      const [pid, rkey] = cloneFromKey.split(":");
      if (pid && rkey) cloneFrom = { project_id: pid, role_key: rkey };
    }

    try {
      const created = await createRole.mutateAsync({
        project_id: projectId,
        role_key: roleKey,
        display_name: displayName.trim() || undefined,
        description: description.trim() || undefined,
        clone_from: cloneFrom,
      });
      toastSuccess(
        "Role created",
        created.display_name ? `"${created.display_name}" is ready to assign.` : undefined,
      );
      onCreated?.({ project_id: projectId, role_key: roleKey });
      onClose();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setConflictMsg(
          `A role with key "${roleKey}" already exists in this project. Pick a different role_key.`,
        );
        return;
      }
      toastError(err instanceof Error ? err.message : "Failed to create role");
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      busy={createRole.isPending}
      labelledBy="create-role-title"
      size="lg"
    >
      <Eyebrow>New role</Eyebrow>
      <h2 id="create-role-title" className="text-lg font-semibold text-on-surface mt-1">
        Create a project role
      </h2>
      <p className="mt-1 text-sm text-on-surface-variant">
        Roles are persisted locally and propagated to Zitadel. Cloning copies
        the source role&apos;s display name and description as starting values.
      </p>

      <form onSubmit={handleSubmit} className="mt-5 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label htmlFor="create-role-project" className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Project
            </label>
            <Select
              id="create-role-project"
              value={projectId}
              disabled={!!initialProjectId}
              onChange={(event) => setProjectId(event.target.value)}
            >
              <option value="" disabled>
                {projects.length === 0 ? "Loading…" : "Select a project"}
              </option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label htmlFor="create-role-clone" className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Clone from <span className="text-on-surface-variant/70">(optional)</span>
            </label>
            <Select
              id="create-role-clone"
              value={cloneFromKey}
              onChange={(event) => setCloneFromKey(event.target.value)}
            >
              <option value="">Don&apos;t clone</option>
              {cloneCandidates.map((r) => (
                <option key={`${r.project_id}:${r.role_key}`} value={`${r.project_id}:${r.role_key}`}>
                  {r.display_name || r.role_key}
                </option>
              ))}
            </Select>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label htmlFor="create-role-display" className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Display name {cloneFromKey ? <span className="text-on-surface-variant/70">(optional — inherits from clone)</span> : null}
            </label>
            <Input
              id="create-role-display"
              placeholder="e.g. Workshop Mentor"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              autoFocus
            />
          </div>
          <div>
            <label htmlFor="create-role-key" className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Role key <span className="text-on-surface-variant/70">(slug)</span>
            </label>
            <Input
              id="create-role-key"
              placeholder="e.g. workshop_mentor"
              value={roleKey}
              onChange={(event) => {
                setKeyTouched(true);
                setRoleKey(event.target.value);
                setConflictMsg(null);
              }}
              aria-invalid={roleKey.length > 0 && !slugValid}
              aria-describedby={roleKey.length > 0 && !slugValid ? "create-role-key-error" : undefined}
            />
            {roleKey.length > 0 && !slugValid && (
              <p id="create-role-key-error" className="mt-1 text-[11px] text-[var(--error)]">
                Lowercase letters, digits, &quot;-&quot;, &quot;_&quot;; must start with a letter.
              </p>
            )}
            {conflictMsg && (
              <p className="mt-1 text-[11px] text-[var(--error)]">{conflictMsg}</p>
            )}
          </div>
        </div>

        <div>
          <label htmlFor="create-role-description" className="block text-xs font-medium text-on-surface-variant mb-1.5">
            Description <span className="text-on-surface-variant/70">(optional)</span>
          </label>
          <Input
            id="create-role-description"
            placeholder="What does this role grant?"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </div>

        <div className="flex items-center justify-end gap-3 pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <SubmitButton
            isPending={createRole.isPending}
            disabled={!canSubmit}
            pendingLabel="Creating…"
            label="Create role"
          />
        </div>
      </form>
    </Modal>
  );
}

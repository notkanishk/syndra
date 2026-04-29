"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { useAddBundleRole, type BundleRoleRow } from "@/lib/queries/useBundles";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import { toastError, toastSuccess } from "@/lib/toast";

interface AddRolesToBundlePickerProps {
  bundleId: string;
  bundleName: string;
  open: boolean;
  onClose: () => void;
  /** Roles already in the bundle — disabled in the picker so we never re-add. */
  existingRoles: BundleRoleRow[];
}

interface PickerKey {
  projectId: string;
  roleKey: string;
}

function keyFor(projectId: string, roleKey: string) {
  return `${projectId}:${roleKey}`;
}

/**
 * Searchable, multi-select role picker for adding roles to a bundle. Replaces
 * the Stage 3 inline (project, role) Select pair with a single browse-and-pick
 * flow grouped by project. Backed by `useGlobalRoleCatalog()` so it surfaces
 * every role the system knows about — local DB, directory source, and roles
 * referenced by existing assignments — without requiring the operator to know
 * the project a role lives in.
 *
 * The mutation runs sequentially because the backend's `POST /bundles/{id}/roles`
 * accepts one (project_id, role_key) per call. Failures stop the loop and the
 * remaining roles stay queued in the UI so the operator can retry without
 * losing their selection.
 */
export default function AddRolesToBundlePicker({
  bundleId,
  bundleName,
  open,
  onClose,
  existingRoles,
}: AddRolesToBundlePickerProps) {
  const catalogQuery = useGlobalRoleCatalog();
  const addRole = useAddBundleRole(bundleId);

  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<Map<string, PickerKey>>(new Map());
  const [pendingCount, setPendingCount] = useState(0);

  // Reset state on close so the next session starts clean.
  useEffect(() => {
    if (!open) {
      setSearch("");
      setSelected(new Map());
      setPendingCount(0);
    }
  }, [open]);

  const existingKeys = useMemo(() => {
    const set = new Set<string>();
    for (const r of existingRoles) set.add(keyFor(r.zitadel_project_id, r.zitadel_role_key));
    return set;
  }, [existingRoles]);

  // Group roles by project_name for a clean two-level list. Filter by search
  // text against project name, role display name, and role key — operators
  // sometimes search by either, so all three feed the predicate.
  const groupedByProject = useMemo(() => {
    const all = catalogQuery.data ?? [];
    const lower = search.trim().toLowerCase();
    const matches = (role: CatalogRole) => {
      if (!lower) return true;
      return (
        role.project_name.toLowerCase().includes(lower) ||
        role.display_name.toLowerCase().includes(lower) ||
        role.role_key.toLowerCase().includes(lower)
      );
    };
    const groups = new Map<string, { projectId: string; projectName: string; roles: CatalogRole[] }>();
    for (const role of all) {
      if (!matches(role)) continue;
      const key = role.project_id;
      const bucket = groups.get(key) ?? {
        projectId: role.project_id,
        projectName: role.project_name,
        roles: [],
      };
      bucket.roles.push(role);
      groups.set(key, bucket);
    }
    // Stable order: project name asc, role display name asc within group.
    return Array.from(groups.values())
      .sort((a, b) => a.projectName.localeCompare(b.projectName))
      .map((bucket) => ({
        ...bucket,
        roles: bucket.roles.slice().sort((a, b) => a.display_name.localeCompare(b.display_name)),
      }));
  }, [catalogQuery.data, search]);

  function toggle(role: CatalogRole) {
    const k = keyFor(role.project_id, role.role_key);
    setSelected((prev) => {
      const next = new Map(prev);
      if (next.has(k)) {
        next.delete(k);
      } else {
        next.set(k, { projectId: role.project_id, roleKey: role.role_key });
      }
      return next;
    });
  }

  async function handleSubmit() {
    if (selected.size === 0) return;
    const queue = Array.from(selected.values());
    setPendingCount(queue.length);
    let added = 0;
    for (const item of queue) {
      try {
        await addRole.mutateAsync({ project_id: item.projectId, role_key: item.roleKey });
        added += 1;
        setPendingCount((p) => p - 1);
      } catch (err) {
        toastError(
          err instanceof Error ? err.message : `Failed to add ${item.projectId}:${item.roleKey}`,
        );
        // Keep the remaining items in the picker so the operator can retry.
        const remaining = new Map(selected);
        const completed = queue.slice(0, added);
        for (const c of completed) remaining.delete(keyFor(c.projectId, c.roleKey));
        setSelected(remaining);
        setPendingCount(0);
        return;
      }
    }
    toastSuccess(
      `Added ${added} role${added === 1 ? "" : "s"} to "${bundleName}"`,
    );
    setPendingCount(0);
    onClose();
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      busy={pendingCount > 0}
      labelledBy="add-roles-title"
      size="lg"
    >
      <Eyebrow>Add roles</Eyebrow>
      <h2 id="add-roles-title" className="text-lg font-semibold text-on-surface mt-1">
        Roles for &ldquo;{bundleName}&rdquo;
      </h2>
      <p className="mt-1 text-sm text-on-surface-variant">
        Search across all known roles — local, discovered, or referenced — and
        select any number to attach in one batch.
      </p>

      <div className="mt-4">
        <Input
          placeholder="Search by project, role name, or role key…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      <div
        className="mt-4 max-h-80 overflow-y-auto rounded-card border border-outline-variant bg-surface-container-low"
        role="listbox"
        aria-multiselectable="true"
        aria-label="Available roles"
      >
        {catalogQuery.isLoading ? (
          <p className="px-4 py-6 text-sm text-on-surface-variant">Loading role catalog…</p>
        ) : groupedByProject.length === 0 ? (
          <p className="px-4 py-6 text-sm text-on-surface-variant">
            No roles match your search.
          </p>
        ) : (
          <ul className="divide-y divide-outline-variant/60">
            {groupedByProject.map((group) => (
              <li key={group.projectId} className="px-4 py-3">
                <Eyebrow tone="muted">{group.projectName}</Eyebrow>
                <ul className="mt-2 grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {group.roles.map((role) => {
                    const k = keyFor(role.project_id, role.role_key);
                    const alreadyAdded = existingKeys.has(k);
                    const isSelected = selected.has(k);
                    return (
                      <li key={k}>
                        <button
                          type="button"
                          role="option"
                          aria-selected={isSelected}
                          aria-disabled={alreadyAdded}
                          disabled={alreadyAdded}
                          onClick={() => toggle(role)}
                          className={`w-full text-left rounded-card border px-3 py-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
                            alreadyAdded
                              ? "border-outline-variant/40 bg-surface-container-low text-on-surface-variant cursor-not-allowed"
                              : isSelected
                                ? "border-primary-container bg-primary-container/10 text-on-surface"
                                : "border-outline-variant bg-surface-container hover:border-primary-container/60 text-on-surface"
                          }`}
                        >
                          <div className="font-medium">{role.display_name || role.role_key}</div>
                          <div className="mt-0.5 text-[11px] text-on-surface-variant font-mono">
                            {role.role_key}
                          </div>
                          {alreadyAdded && (
                            <div className="mt-1 text-[11px] text-[var(--success)]">
                              Already in bundle
                            </div>
                          )}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </li>
            ))}
          </ul>
        )}
      </div>

      {selected.size > 0 && (
        <div className="mt-4">
          <Eyebrow tone="primary">Selected ({selected.size})</Eyebrow>
          <div className="mt-2 flex flex-wrap gap-2">
            {Array.from(selected.entries()).map(([k]) => (
              <Badge key={k} variant="outline" className="border-primary-container/40 text-primary-container">
                {k}
              </Badge>
            ))}
          </div>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3">
        <p className="text-xs text-on-surface-variant">
          {pendingCount > 0
            ? `Adding ${pendingCount} more…`
            : `${existingRoles.length} role${existingRoles.length === 1 ? "" : "s"} already in this bundle`}
        </p>
        <div className="flex items-center gap-3">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <SubmitButton
            isPending={pendingCount > 0}
            disabled={selected.size === 0}
            pendingLabel="Adding…"
            label={selected.size === 0 ? "Select roles" : `Add ${selected.size} role${selected.size === 1 ? "" : "s"}`}
            onClick={handleSubmit}
            type="button"
          />
        </div>
      </div>
    </Modal>
  );
}

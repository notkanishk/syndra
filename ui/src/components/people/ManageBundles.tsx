"use client";

import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { RoleRef } from "@/components/names";
import { Button } from "@/components/ui/Button";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useBundleRolesByBundle, useBundles, useRemoveBundle } from "@/lib/queries/useBundles";
import { useMappingRules } from "@/lib/queries/useMappingRules";
import { useAssignBundle, useUserAccess } from "@/lib/queries/useUsers";

/**
 * E4 · Assign / unassign a bundle.
 *
 * The preview is the body of the panel, not a footnote: bundles expand to
 * roles and rules cascade further, so "what would this actually grant" has to
 * be answerable before the click, above the fold.
 *
 * Unassigning shows the same preview in reverse and distinguishes roles that
 * will actually be lost from roles retained through another source.
 */
export function ManageBundles({
  userId,
  userName,
  assigned,
  open,
  onClose,
}: {
  userId: string;
  userName: string;
  assigned: Array<{ id: string; name: string }>;
  open: boolean;
  onClose: () => void;
}) {
  const bundles = useBundles();
  const access = useUserAccess(userId);
  const rules = useMappingRules();
  const assign = useAssignBundle(userId);
  const remove = useRemoveBundle(userId);

  const assignedIds = useMemo(() => new Set(assigned.map((b) => b.id)), [assigned]);
  const [staged, setStaged] = useState<Set<string>>(new Set());
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const allIds = (bundles.data ?? []).map((bundle) => bundle.id);
  const { byId: bundleRoles } = useBundleRolesByBundle(allIds);

  const selected = useMemo(() => {
    const next = new Set(assignedIds);
    staged.forEach((id) => (next.has(id) ? next.delete(id) : next.add(id)));
    return next;
  }, [assignedIds, staged]);

  const changes = Array.from(staged);
  const previewFor = changes[0];

  const held = useMemo(() => {
    const keys = new Set<string>();
    for (const project of access.data?.projects ?? []) {
      for (const key of project.effective_role_keys) keys.add(`${project.project_id}::${key}`);
    }
    return keys;
  }, [access.data]);

  if (!open) return null;

  const previewBundle = (bundles.data ?? []).find((bundle) => bundle.id === previewFor);
  const previewAdding = previewFor ? !assignedIds.has(previewFor) : false;
  const previewRoles = previewFor ? (bundleRoles[previewFor] ?? []) : [];

  async function apply() {
    try {
      for (const id of changes) {
        if (assignedIds.has(id)) await remove.mutateAsync(id);
        else await assign.mutateAsync(id);
      }
      // Queued, not applied: a bundle assignment reaches a target through the
      // drain, and `summary.succeeded` is always zero precisely so a client
      // cannot report otherwise.
      setOutcome({
        kind: "queued",
        message:
          changes.length === 1
            ? "One bundle change recorded"
            : `${changes.length} bundle changes recorded`,
        detail: "Nothing has reached a target yet — the writes dispatch on the next drain.",
      });
      setStaged(new Set());
    } catch (error) {
      setOutcome(outcomeFromError(error));
    }
  }

  const busy = assign.isPending || remove.isPending;

  return (
    <Modal open onClose={onClose} busy={busy} size="md" labelledBy="manage-bundles-title">
      <ModalHeader title="Manage bundles" titleId="manage-bundles-title" />
      <div className="-mt-2 px-6 pb-1 text-[13px] text-faint">{userName}</div>

      <div className="flex flex-col gap-2.5 px-6">
        {(bundles.data ?? []).map((bundle) => {
          const isSelected = selected.has(bundle.id);
          const roleCount = (bundleRoles[bundle.id] ?? []).length;
          return (
            <button
              key={bundle.id}
              type="button"
              role="checkbox"
              aria-checked={isSelected}
              onClick={() =>
                setStaged((prev) => {
                  const next = new Set(prev);
                  if (next.has(bundle.id)) next.delete(bundle.id);
                  else next.add(bundle.id);
                  return next;
                })
              }
              className={`flex items-center gap-3 rounded-inner border px-[15px] py-3 text-left motion-tint ${
                isSelected
                  ? "border-accent-line bg-accent-soft/70"
                  : "border-line-strong hover:bg-[var(--hover)]"
              }`}
            >
              <span
                aria-hidden
                className={`h-[18px] w-[18px] flex-none rounded-[6px] ${
                  isSelected ? "bg-accent" : "border-[1.5px] border-ink/35"
                }`}
              />
              <span className="flex-1 text-[15px] font-semibold">{bundle.name}</span>
              <span className="text-[13.5px] text-muted">
                {roleCount} {roleCount === 1 ? "role" : "roles"}
                {assignedIds.has(bundle.id) ? " · assigned" : ""}
              </span>
              {bundle.is_welcome && (
                <Badge>Default for new members</Badge>
              )}
            </button>
          );
        })}
      </div>

      {previewBundle && (
        <div className="mx-6 mt-5 rounded-block border border-line bg-tint-1 px-[18px] py-4">
          <div className="mb-2.5 type-label">
            {previewAdding ? `Adding ${previewBundle.name} would grant` : `Removing ${previewBundle.name} would take away`}
          </div>
          <div className="flex flex-col gap-[7px] text-[14px]">
            {previewRoles.length === 0 && (
              <span className="text-faint">This bundle carries no roles yet.</span>
            )}
            {previewRoles.map((role) => {
              const projectId = role.zitadel_project_id;
              const roleKey = role.zitadel_role_key;
              const alreadyHeld = held.has(`${projectId}::${roleKey}`);
              const cascade = (rules.data ?? []).find(
                (rule) => rule.source_project === projectId && rule.source_role === roleKey,
              );
              return (
                <div key={`${projectId}:${roleKey}`} className="flex flex-col gap-[7px]">
                  <div
                    className={`flex items-center gap-2.5 ${
                      previewAdding && alreadyHeld ? "text-faint" : ""
                    }`}
                  >
                    <span
                      aria-hidden
                      className={`h-1.5 w-1.5 rounded-pill ${
                        previewAdding && alreadyHeld ? "bg-ink/30" : "bg-accent"
                      }`}
                    />
                    <RoleRef projectId={projectId} roleKey={roleKey} />
                    {previewAdding && alreadyHeld && (
                      <span className="text-[13px]">— already held, no change</span>
                    )}
                  </div>
                  {cascade && (
                    <div className="flex items-center gap-2.5 text-muted">
                      <span
                        aria-hidden
                        className="h-1.5 w-1.5 rounded-pill border border-dashed border-ink/60"
                      />
                      and cascade to <RoleRef projectId={cascade.target_project} roleKey={cascade.target_role} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          changes.length > 1
            ? "Each change is applied in order. Any that fail stay unapplied and are reported."
            : undefined
        }
      >
        <Button variant="accent" disabled={changes.length === 0} isPending={busy} onClick={apply}>
          {changes.length === 0
            ? "No changes"
            : `Apply ${changes.length} ${changes.length === 1 ? "change" : "changes"}`}
        </Button>
        <Button onClick={onClose}>{outcome?.kind === "queued" ? "Done" : "Cancel"}</Button>
        <span className="flex-1" />
        <span className="text-[13px] text-faint">Queues for confirmation</span>
      </ModalFooter>
    </Modal>
  );
}

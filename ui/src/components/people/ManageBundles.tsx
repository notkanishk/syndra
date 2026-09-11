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
 *
 * It reads the PUBLISHED version of each bundle, and that is not a detail. An
 * assignment pins the latest published version and nothing else — the backend
 * says so at the read (`db.LatestVersionRoles`: "Not the working copy either.
 * Reading `bundle_roles` would project unpublished edits to somebody who is not
 * pinned to them") and a test pins the apply side to it. This panel asked for
 * the working copy, so it listed every unpublished edit as a role the person
 * was about to receive, and the apply then granted the published set. An
 * operator saw the roles they had just added promised here and called
 * unpublished on the bundle screen, which is two screens disagreeing about the
 * same fact.
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
  const { byId: bundleRoles } = useBundleRolesByBundle(allIds, { published: true });

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
      // What is WAITING afterwards, summed from the backend rather than
      // assumed from the number of boxes ticked. The two are different every
      // time a change leaves nothing to send: a removal whose delivery had
      // never gone out withdraws the queued rows instead of queueing their
      // opposite, and a change that reaches nobody's effective access queues
      // nothing either. Both used to be reported as "the changes wait under
      // Pending changes until you send them" — an instruction to go and
      // confirm an empty screen.
      let waiting = 0;
      for (const id of changes) {
        const result = assignedIds.has(id)
          ? await remove.mutateAsync(id)
          : await assign.mutateAsync(id);
        waiting += result?.cascade?.enqueued ?? 0;
      }
      const recorded =
        changes.length === 1 ? "One bundle change recorded" : `${changes.length} bundle changes recorded`;
      setOutcome({
        // Amber "waiting" only where something waits. A change that queued
        // nothing is done, and painting it as pending sends an operator to an
        // empty Pending changes to look for it.
        kind: waiting > 0 ? "queued" : "applied",
        message: recorded,
        detail:
          waiting > 0
            ? `Nothing has reached Zitadel or a connected system yet. ${
                waiting === 1 ? "One change waits" : `${waiting} changes wait`
              } under Pending changes until you send them.`
            : "Nothing is waiting to be sent: this left no change for Zitadel or a connected system to carry out. Pending changes is unaffected.",
      });
      setStaged(new Set());
    } catch (error) {
      setOutcome(outcomeFromError(error));
    }
  }

  const busy = assign.isPending || remove.isPending;
  const staging = changes.length;
  // Recorded, and nothing new ticked since. The modal has nothing left to do.
  const done = (outcome?.kind === "queued" || outcome?.kind === "applied") && staging === 0;

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
            {previewAdding ? `Adding ${previewBundle.name} would grant` : `Removing ${previewBundle.name} would revoke`}
          </div>
          <div className="flex flex-col gap-[7px] text-[14px]">
            {previewRoles.length === 0 && (
              <span className="text-faint">
                {previewBundle.latest_version
                  ? `v${previewBundle.latest_version} of this bundle carries no roles, so this grants nothing.`
                  : "This bundle carries no roles yet."}
              </span>
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
                      and, through an automatic rule, <RoleRef projectId={cascade.target_project} roleKey={cascade.target_role} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/*
            Said here, on the screen that would otherwise be contradicted. A
            bundle with unpublished edits grants its published version, so this
            list is right and the bundle screen's role list is a different set —
            and an operator who has just added those roles will read their
            absence as this panel being stale rather than as the truth about
            what an assignment pins.
          */}
          {previewAdding && (previewBundle.unpublished_changes ?? 0) > 0 && (
            <p className="mt-3 border-t border-line pt-2.5 text-[13px] leading-[1.5] text-muted">
              {previewBundle.name} has {previewBundle.unpublished_changes} unpublished{" "}
              {previewBundle.unpublished_changes === 1 ? "change" : "changes"}. Those are not part
              of this — an assignment gives v{previewBundle.latest_version}, and publishing is
              what decides whether the people holding it move.
            </p>
          )}
        </div>
      )}

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      {/* The footer follows the state of the work, because the work has three
          states and this used to describe only the middle one.

          "Queues for confirmation" sat permanently in the corner — true while
          something is staged, and a claim about a button that cannot be pressed
          both before anything is ticked and after the change has been recorded.
          Beside it the primary read "No changes": a status wearing a button's
          clothes, disabled, giving no reason, and still first in the tab order
          while the only live control was the secondary one next to it. */}
      <ModalFooter
        note={
          changes.length > 1
            ? "Each change is applied in order. Any that fail stay unapplied and are reported."
            : undefined
        }
      >
        {done ? (
          // The work is recorded and the outcome above says so. The next action
          // is to leave, so it stops being the quiet button in the corner.
          <Button variant="accent" onClick={onClose}>
            Done
          </Button>
        ) : (
          <>
            <Button
              variant="accent"
              disabled={staging === 0}
              isPending={busy}
              onClick={apply}
              // Never a bare disabled control. The label stays an ACTION at
              // every count — a button labelled with its own emptiness reads
              // as broken rather than as waiting for you.
              reason={
                staging === 0
                  ? outcome
                    ? "Recorded. Tick another bundle to make a further change."
                    : "Tick a bundle above to add or remove it."
                  : undefined
              }
            >
              {staging === 0
                ? "Apply changes"
                : `Apply ${staging} ${staging === 1 ? "change" : "changes"}`}
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </>
        )}
        <span className="flex-1" />
        {staging > 0 && (
          // Only where it is true: this button records the change and sends
          // nothing. With nothing staged it was describing an act that was not
          // on offer.
          //
          // "Queues for confirmation" used to sit here, and it isn't always
          // true — a change that reaches nobody's effective access is
          // reported as `applied` with nothing queued at all (see `apply()`
          // above), so a caption promising a queue every time contradicted
          // that outcome. "Recorded" is true in both cases; the outcome
          // above states which one this was.
          <span className="text-[13px] text-faint">Recorded in Syndra first, not sent yet</span>
        )}
      </ModalFooter>
    </Modal>
  );
}

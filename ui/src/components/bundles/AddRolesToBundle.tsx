"use client";

import { useMemo, useState } from "react";

import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { Button } from "@/components/ui/Button";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { type ActionOutcome as Outcome } from "@/lib/outcome";
import { useAddBundleRole, useBundleRoles } from "@/lib/queries/useBundles";

/**
 * Adding roles to a bundle, as the job it actually is.
 *
 * The control this replaces was a project select, then a role select, then a
 * button — one round trip per role, and no way to see what a bundle was
 * missing without walking the projects one at a time. Building a bundle is
 * normally "give this bundle the six things a new member needs", which is one
 * decision, not six. The list itself is `RolePicker`, shared with the create
 * dialog, which needs the same control for the same reason.
 *
 * What this owns is the part that is specific to editing an existing bundle:
 * **a partial failure is resumable.** The API takes one role per call, so N
 * roles is N writes; if the fourth fails the first three stay added and the
 * rest stay selected, with the failure named. Clearing the selection on error
 * would make the operator reconstruct what they had asked for.
 */
export function AddRolesToBundle({
  bundleId,
  name,
  holders,
  onClose,
}: {
  bundleId: string;
  name: string;
  holders: number;
  onClose: () => void;
}) {
  const existing = useBundleRoles(bundleId);
  const addRole = useAddBundleRole(bundleId);

  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  const [failure, setFailure] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const held = useMemo(
    () =>
      new Set(
        (existing.data ?? []).map(
          (role) => `${role.zitadel_project_id}:${role.zitadel_role_key}`,
        ),
      ),
    [existing.data],
  );

  const chosen = Array.from(selected);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  async function apply() {
    setApplying(true);
    setFailure(null);
    let added = 0;
    // Sequential, not concurrent. Each add is its own write against the
    // working copy, and six at once would interleave into a draft nobody
    // asked for if one of them failed halfway.
    for (const id of chosen) {
      const [projectId, roleKey] = splitRoleId(id);
      try {
        await addRole.mutateAsync({ project_id: projectId, role_key: roleKey });
        added += 1;
        setSelected((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      } catch (error) {
        setFailure(
          error instanceof Error ? error.message : `${roleKey} couldn't be added to ${name}.`,
        );
        // `break`, and nothing else. This used to set a local flag because the
        // dialog closed itself on success and a failed apply must not be
        // closed over — the flag outlived the auto-close, and a variable that
        // is assigned and never read is a gate that guards nothing. What keeps
        // the failure visible now is that nothing closes this dialog but the
        // operator.
        break;
      }
    }
    setApplying(false);

    if (added > 0) {
      setOutcome({
        // `no_change` about access, deliberately: the working copy moved and
        // nobody's access did. Reporting this as `applied` is the misreading
        // the removal panel used to invite, in the opposite direction.
        kind: "no_change",
        message: `${added} ${added === 1 ? "role" : "roles"} added to the ${name} draft`,
        detail:
          holders > 0
            ? `Nobody has them yet. Publish a version to decide whether the ${holders} ${
                holders === 1 ? "person" : "people"
              } holding ${name} ${holders === 1 ? "gets" : "get"} them.`
            : "Publish a version to give them to people.",
      });
    }
  }

  return (
    <Modal open onClose={onClose} busy={applying} size="md" labelledBy="add-roles-title">
      <ModalHeader
        title={`Add roles to ${name}`}
        titleId="add-roles-title"
        lede={
          holders > 0
            ? `Roles you tick go into the draft. The ${holders} ${
                holders === 1 ? "person who holds" : "people who hold"
              } ${name} keep${holders === 1 ? "s" : ""} exactly what they have until you publish a version and move them onto it.`
            : "Roles you tick go into the draft. Publish a version to give them to people."
        }
      />

      <div className="px-6">
        <RolePicker
          selected={selected}
          onToggle={toggle}
          lockedIds={held}
          lockedNote={`already in ${name}`}
          disabled={applying}
        />
      </div>

      {failure && (
        // Named, and the rest stay selected. A partial apply that reports only
        // "something went wrong" leaves the operator unable to tell which of
        // the six landed.
        <div className="danger-note mx-6 mt-3 px-4 py-3 text-[14px] leading-[1.5]">
          {failure} The roles still ticked were not added — press Add {chosen.length}{" "}
          {chosen.length === 1 ? "role" : "roles"} again to add the rest.
        </div>
      )}

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={chosen.length === 0 || applying}
          isPending={applying}
          onClick={apply}
        >
          {chosen.length === 0
            ? "Add roles"
            : `Add ${chosen.length} ${chosen.length === 1 ? "role" : "roles"}`}
        </Button>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}

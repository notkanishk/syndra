"use client";

import { useMemo, useState } from "react";
import { describeFailure, sentence } from "@/lib/outcome";

import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { Button } from "@/components/ui/Button";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { type ActionOutcome as Outcome } from "@/lib/outcome";
import { useBundleDraft } from "@/lib/queries/useBundleVersions";
import { useAddBundleRole, useBundleRoles, useSetBundleInclude } from "@/lib/queries/useBundles";

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
  const include = useSetBundleInclude(bundleId);
  const draft = useBundleDraft(bundleId);

  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  // Bundles to include (B9): each goes into the next version at its latest version.
  const [bundles, setBundles] = useState<ReadonlySet<string>>(() => new Set());
  const includedNow = useMemo(
    () => new Set((draft.data?.includes ?? []).map((i) => i.child_bundle_id)),
    [draft.data?.includes],
  );
  const [applying, setApplying] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [word, setWord] = useState("Added");

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
  const chosenBundles = Array.from(bundles);
  const count = chosen.length + chosenBundles.length;
  const what = whatOf(chosen.length, chosenBundles.length);

  function toggle(id: string) {
    setSelected((prev) => toggled(prev, id));
  }

  async function apply() {
    setApplying(true);
    // One result slot: the press clears it before anything is written (A-22 G23).
    setOutcome(null);
    let failed: unknown = null;
    let added = 0;
    let included = 0;
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
        failed = error;
        // `break`, and nothing else. This used to set a local flag because the
        // dialog closed itself on success and a failed apply must not be
        // closed over — the flag outlived the auto-close, and a variable that
        // is assigned and never read is a gate that guards nothing. What keeps
        // the failure visible now is that nothing closes this dialog but the
        // operator.
        break;
      }
    }
    // Then the bundles, the same way: one write each, stopping at a refusal
    // (a bundle can be refused for nesting) with the rest still chosen.
    for (const id of added === chosen.length ? chosenBundles : []) {
      try {
        await include.mutateAsync({ child: id });
        included += 1;
        setBundles((prev) => toggled(prev, id));
      } catch (error) {
        failed = error;
        break;
      }
    }
    setApplying(false);

    if (failed !== null) {
      // One block, named, and the rest stay ticked: a partial apply that reports only "something
      // went wrong" leaves the operator unable to tell which of the six landed (A-22 §3.18).
      const done = added + included;
      const rest = whatOf(chosen.length - added, added === chosen.length ? chosenBundles.length - included : chosenBundles.length);
      setWord(done === 0 ? "Not added" : `Added ${done} of ${chosen.length + chosenBundles.length}`);
      setOutcome({
        kind: "failed",
        partial: `${name}'s next version`,
        message: sentence(describeFailure(failed)),
        detail: done === 0 ? "Nothing was added. What is ticked is still ticked." : "What is still ticked was not added.",
        next: done === 0 ? `Press Add ${rest} again.` : `Press Add ${rest} again for the rest.`,
      });
      return;
    }
    setWord("Added");

    if (included > 0 && added === 0) {
      setOutcome({
        kind: "no_change",
        message: `${included} ${included === 1 ? "bundle" : "bundles"} added to ${name}'s next version`,
        detail:
          holders > 0
            ? `Publish a version to decide whether the ${holders} ${
                holders === 1 ? "person" : "people"
              } ${name} is assigned to ${holders === 1 ? "gets" : "get"} ${included === 1 ? "it" : "them"}.`
            : "Publish a version to give it to people.",
      });
    } else if (added > 0) {
      setOutcome({
        // `no_change` about access, deliberately: the working copy moved and
        // nobody's access did. Reporting this as `applied` is the misreading
        // the removal panel used to invite, in the opposite direction.
        kind: "no_change",
        message: `${[
          included ? `${included} ${included === 1 ? "bundle" : "bundles"}` : null,
          `${added} ${added === 1 ? "role" : "roles"}`,
        ]
          .filter(Boolean)
          .join(" and ")} added to ${name}'s next version`,
        detail:
          holders > 0
            ? `Publish a version to decide whether the ${holders} ${
                holders === 1 ? "person" : "people"
              } ${name} is assigned to ${holders === 1 ? "gets" : "get"} them.`
            : "Publish a version to give them to people.",
      });
    }
  }

  return (
    <Modal open onClose={onClose} busy={applying} size="md" labelledBy="add-roles-title">
      <ModalHeader
        title={`Add roles to ${name}`}
        titleId="add-roles-title"
        lede="Roles you tick go into the next version."
      />

      <div className="px-6">
        <RolePicker
          selected={selected}
          onToggle={toggle}
          lockedIds={held}
          lockedNote={`already in ${name}`}
          disabled={applying}
          bundles={{
            selfId: bundleId,
            selected: bundles,
            onToggle: (id) => setBundles((prev) => toggled(prev, id)),
            locked: includedNow,
            lockedNote: `already in ${name}`,
          }}
        />
      </div>

      {outcome && <ActionOutcome outcome={outcome} word={word} className="mx-6 mb-1" />}

      <ModalFooter cancel={outcome ? undefined : { onClick: onClose, disabled: applying }}>
        {/* A work sheet: Add stays beside Close while anything is still ticked. */}
        {(!outcome || count > 0) && (
          <Button
            variant="accent"
            disabled={count === 0 || applying}
            isPending={applying}
            onClick={apply}
          >
            {count === 0 ? "Add roles" : `Add ${what}`}
          </Button>
        )}
        {outcome && (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

function whatOf(roles: number, bundles: number): string {
  return [
    bundles ? `${bundles} ${bundles === 1 ? "bundle" : "bundles"}` : null,
    roles ? `${roles} ${roles === 1 ? "role" : "roles"}` : null,
  ]
    .filter(Boolean)
    .join(" and ");
}

function toggled(prev: ReadonlySet<string>, id: string): ReadonlySet<string> {
  const next = new Set(prev);
  if (!next.delete(id)) next.add(id);
  return next;
}

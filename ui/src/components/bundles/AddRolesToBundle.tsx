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
  // Bundles to include (B9): each goes into the working copy at its latest version.
  const [bundles, setBundles] = useState<ReadonlySet<string>>(() => new Set());
  const includedNow = useMemo(
    () => new Set((draft.data?.includes ?? []).map((i) => i.child_bundle_id)),
    [draft.data?.includes],
  );
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
  const chosenBundles = Array.from(bundles);
  const count = chosen.length + chosenBundles.length;
  const what = [
    chosenBundles.length ? `${chosenBundles.length} ${chosenBundles.length === 1 ? "bundle" : "bundles"}` : null,
    chosen.length ? `${chosen.length} ${chosen.length === 1 ? "role" : "roles"}` : null,
  ]
    .filter(Boolean)
    .join(" and ");

  function toggle(id: string) {
    setSelected((prev) => toggled(prev, id));
  }

  async function apply() {
    setApplying(true);
    setFailure(null);
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
        setFailure(sentence(describeFailure(error)));
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
        setFailure(sentence(describeFailure(error)));
        break;
      }
    }
    setApplying(false);

    if (included > 0 && added === 0) {
      setOutcome({
        kind: "no_change",
        message: `${included} ${included === 1 ? "bundle" : "bundles"} added to the ${name} draft`,
        detail:
          holders > 0
            ? `Nobody has what ${included === 1 ? "it gives" : "they give"} yet. Publish a version to decide whether the ${holders} ${
                holders === 1 ? "person" : "people"
              } holding ${name} ${holders === 1 ? "gets" : "get"} it.`
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
          .join(" and ")} added to the ${name} draft`,
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
          bundles={{
            selfId: bundleId,
            selected: bundles,
            onToggle: (id) => setBundles((prev) => toggled(prev, id)),
            locked: includedNow,
            lockedNote: `already in ${name}`,
          }}
        />
      </div>

      {failure && (
        // Named, and the rest stay selected. A partial apply that reports only
        // "something went wrong" leaves the operator unable to tell which of
        // the six landed.
        <div className="danger-note mx-6 mt-3 px-4 py-3 text-[14px] leading-[1.5]">
          {failure} What is still ticked was not added — press Add {what} again to add the rest.
        </div>
      )}

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={count === 0 || applying}
          isPending={applying}
          onClick={apply}
        >
          {count === 0 ? "Add roles" : `Add ${what}`}
        </Button>
        <Button onClick={onClose}>Close</Button>
      </ModalFooter>
    </Modal>
  );
}

function toggled(prev: ReadonlySet<string>, id: string): ReadonlySet<string> {
  const next = new Set(prev);
  if (!next.delete(id)) next.add(id);
  return next;
}

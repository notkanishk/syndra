"use client";

import { useState } from "react";

import { RoleRef } from "@/components/names";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import {
  useApplyPublish,
  useRehearsePublish,
  type BundleDraft,
} from "@/lib/queries/useBundleVersions";

/**
 * Publishing a version — the step where an edit acquires consequences.
 *
 * The compose step asks one question, and it is the question the whole feature
 * exists for: do the people already holding this bundle come along. Both
 * answers are legitimate. "Leave them" is what you pick when a bundle has been
 * reshaped for the next intake and the current cohort should finish the term on
 * what they were given; "move everyone" is what you pick the rest of the time.
 *
 * The choice is part of the COMPOSE step rather than a toggle on the result,
 * because it changes the plan. Rehearsing tells you what moving fourteen people
 * would do; flipping the answer afterwards would leave a plan on screen that
 * describes something else.
 */
export function PublishVersionDialog({
  bundleId,
  name,
  draft,
  onClose,
}: {
  bundleId: string;
  name: string;
  draft: BundleDraft;
  onClose: () => void;
}) {
  const rehearse = useRehearsePublish(bundleId);
  const apply = useApplyPublish(bundleId);

  const [note, setNote] = useState("");
  const [migrate, setMigrate] = useState<boolean | null>(null);

  const holders = draft.holder_count;
  // With nobody holding it there is no question to ask, so there is no question
  // asked: publishing is unambiguous and the compose step is just the note.
  const decided = holders === 0 ? true : migrate !== null;
  const willMigrate = holders === 0 ? false : migrate === true;

  return (
    <RehearsalDialog
      title={`Publish ${name} v${draft.next_version}`}
      lede={
        holders === 0
          ? "Nobody holds this bundle yet, so publishing changes nothing for anyone."
          : `${holders} ${holders === 1 ? "person holds" : "people hold"} this bundle today. Publishing does not move them unless you say so.`
      }
      noun={["person", "people"]}
      ready={decided}
      destructive={willMigrate && draft.removed.length > 0}
      compose={
        <div className="flex flex-col gap-4">
          <div>
            <FieldLabel>What v{draft.next_version} contains</FieldLabel>
            <div className="mt-1.5 flex flex-col gap-1.5 rounded-inner border border-line-strong px-4 py-3 text-[14px]">
              {draft.added.map((role) => (
                <div
                  key={`+${role.zitadel_project_id}:${role.zitadel_role_key}`}
                  className="flex items-baseline gap-2"
                >
                  <span className="font-semibold text-accent-text">+</span>
                  <RoleRef
                    projectId={role.zitadel_project_id}
                    roleKey={role.zitadel_role_key}
                  />
                </div>
              ))}
              {draft.removed.map((role) => (
                <div
                  key={`-${role.zitadel_project_id}:${role.zitadel_role_key}`}
                  className="flex items-baseline gap-2"
                >
                  <span className="font-semibold text-danger-text">−</span>
                  <RoleRef
                    projectId={role.zitadel_project_id}
                    roleKey={role.zitadel_role_key}
                  />
                </div>
              ))}
            </div>
            <FieldHint>
              Everything else in v{draft.latest_version} carries over unchanged.
            </FieldHint>
          </div>

          {holders > 0 && (
            <div>
              <FieldLabel>The {holders} who already hold it</FieldLabel>
              <div className="mt-1.5 flex flex-col gap-2">
                <Choice
                  selected={migrate === true}
                  onSelect={() => setMigrate(true)}
                  title={`Move everyone to v${draft.next_version}`}
                  detail="Their access changes to match the new version. The plan below says what that does to each of them."
                />
                <Choice
                  selected={migrate === false}
                  onSelect={() => setMigrate(false)}
                  title={`Leave them on the version they are on`}
                  detail={`v${draft.next_version} applies to new assignments only. Nobody's access changes, and you can move them later.`}
                />
              </div>
            </div>
          )}

          <div>
            <FieldLabel htmlFor="version-note">Why? (optional)</FieldLabel>
            <Input
              id="version-note"
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="Laser sign-off moved into the induction bundle"
            />
            <FieldHint>
              Sits on the version forever. &ldquo;v3&rdquo; means nothing in six months;
              &ldquo;v3 — laser sign-off moved in&rdquo; does.
            </FieldHint>
          </div>
        </div>
      }
      onRehearse={async () => (await rehearse.mutateAsync({ note, migrate: willMigrate })).plan}
      onApply={async () => (await apply.mutateAsync({ note, migrate: willMigrate })).plan}
      onClose={onClose}
    />
  );
}

/**
 * A radio in everything but markup. Both answers are real, so neither is
 * pre-selected: a default here would be the product making the decision the
 * operator opened this dialog to make.
 */
function Choice({
  selected,
  onSelect,
  title,
  detail,
}: {
  selected: boolean;
  onSelect: () => void;
  title: string;
  detail: string;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={`rounded-inner border px-4 py-3 text-left motion-tint ${
        selected ? "border-accent bg-accent-soft" : "border-line-strong hover:bg-[var(--hover)]"
      }`}
    >
      <span className="block text-[14.5px] font-semibold">{title}</span>
      <span className="mt-0.5 block text-[13px] leading-[1.5] text-muted">{detail}</span>
    </button>
  );
}

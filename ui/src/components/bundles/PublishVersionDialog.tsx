"use client";

import { useState } from "react";

import { RoleRef } from "@/components/names";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { formatList } from "@/lib/format";
import {
  useApplyPublish,
  useRehearsePublish,
  type BundleDraft,
  type BundleIncluder,
  type IncludeChange,
  type IncluderAction,
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
  draft: live,
  includers = [],
  onClose,
}: {
  bundleId: string;
  name: string;
  draft: BundleDraft;
  /**
   * The bundles that include this one (B8). Each must be answered, Update or
   * Leave, with no default: Update puts the new version in its working copy,
   * and its own publish moves its people.
   */
  includers?: BundleIncluder[];
  onClose: () => void;
}) {
  const rehearse = useRehearsePublish(bundleId);
  const apply = useApplyPublish(bundleId);

  /**
   * The draft this dialog opened on, held still.
   *
   * A successful publish invalidates the draft query, so the prop this dialog
   * renders from moves the moment the write lands — and every version number on
   * screen moves with it. The result step then reported "Publish Lab Tech v3"
   * for the publish that had just created v2: the one step whose job is to say
   * what happened, naming something that did not.
   *
   * Nothing else edits a bundle while this modal is open, so freezing costs
   * nothing and the dialog stays about the version it is publishing.
   */
  const [draft] = useState(live);

  const [note, setNote] = useState("");
  const [migrate, setMigrate] = useState<boolean | null>(null);
  const [answers, setAnswers] = useState<Record<string, IncluderAction>>({});
  const answered = includers.every((i) => answers[i.bundle_id]);
  const includerAnswers = includers.length
    ? { includers: includers.map((i) => ({ bundle_id: i.bundle_id, action: answers[i.bundle_id] })) }
    : {};
  const through = includers.reduce((n, i) => n + i.people, 0);

  const holders = draft.holder_count;
  // With nobody holding it there is no question to ask, so there is no question
  // asked: publishing is unambiguous and the compose step is just the note.
  const decided = holders === 0 ? true : migrate !== null;
  const willMigrate = holders === 0 ? false : migrate === true;
  // One phrase, used by the field label, the radiogroup's accessible name and
  // the not-ready reason. Written out once because it was written out three
  // times and only one of them agreed with itself for a single holder.
  const holdersPhrase = includers.length
    ? `The ${holders} who ${holders === 1 ? "has" : "have"} it directly`
    : `The ${holders} who already ${holders === 1 ? "holds" : "hold"} it`;
  const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;

  return (
    <RehearsalDialog
      title={`Publish ${name} v${draft.next_version}`}
      lede={
        includers.length > 0
          ? `${name} reaches ${people(holders + through)}: ${holders} directly, and ${through} through ${formatList(
              includers.map((i) => i.name),
            )}, which ${includers.length === 1 ? "includes" : "include"} it. Nobody moves unless you say so.`
          : holders === 0
          ? "Nobody holds this bundle yet, so publishing changes nothing for anyone."
          : `${holders} ${holders === 1 ? "person holds" : "people hold"} this bundle today. Publishing does not move them unless you say so.`
      }
      noun={["person", "people"]}
      ready={decided && answered}
      notReadyReason={
        includers.length === 0
          ? `Choose what happens to the ${holders} ${
              holders === 1 ? "person who already holds" : "people who already hold"
            } it. Both answers are real, so there is no default.`
          : holders > 0
            ? `Choose what happens to the ${people(holders)} and to each bundle. Every answer is real, so there are no defaults.`
            : "Choose what happens to each bundle that includes it. Every answer is real, so there are no defaults."
      }
      // Publishing is an act on the BUNDLE. Two publishes move nobody and are
      // both legitimate — one where nothing holds it yet, and one where the
      // operator has chosen to leave the current holders where they are — and
      // without this the only screen that can cut a version left Apply disabled
      // for both.
      definitionLabel={`Publish v${draft.next_version}`}
      destructive={willMigrate && draft.removed.length > 0}
      compose={
        <div className="flex flex-col gap-4">
          <div>
            <FieldLabel>What changes in v{draft.next_version}</FieldLabel>
            <div className="mt-1.5 flex flex-col gap-1.5 rounded-inner border border-line-strong px-4 py-3 text-[14px]">
              {(draft.include_changes ?? []).map((change) => (
                <div key={`i${change.child_bundle_id}`} className="flex items-baseline gap-2">
                  <IncludeChangeLine change={change} />
                </div>
              ))}
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
                  <span className="font-semibold text-warn-text">−</span>
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
              <FieldLabel>{holdersPhrase}</FieldLabel>
              <div
                role="radiogroup"
                aria-label={holdersPhrase}
                className="mt-1.5 flex flex-col gap-2"
              >
                <Choice
                  selected={migrate === true}
                  onSelect={() => setMigrate(true)}
                  title={`Move everyone to v${draft.next_version}`}
                  detail="Their access changes to match the new version. The preview below lists what that does to each of them."
                />
                <Choice
                  selected={migrate === false}
                  onSelect={() => setMigrate(false)}
                  title={`Leave them on the version they are on`}
                  detail={`v${draft.next_version} goes only to people given this bundle from now on. Nobody's access changes, and you can move them later.`}
                />
              </div>
            </div>
          )}

          {includers.length > 0 && (
            <div>
              <FieldLabel>
                The {includers.length} {includers.length === 1 ? "bundle that includes" : "bundles that include"} it
              </FieldLabel>
              <div className="mt-1.5 rounded-inner border border-dashed border-line-strong px-4">
                {includers.map((includer) => (
                  <div key={includer.bundle_id} className="flex flex-wrap items-center gap-3 border-t border-line py-2.5 first:border-t-0">
                    <span className="flex min-w-[min(200px,100%)] flex-1 flex-col gap-px">
                      <span className="text-[14.5px] font-semibold">{includer.name}</span>
                      <span className="text-[12.5px] text-muted">
                        {people(includer.people)} · includes v{includer.included_version || includer.draft_version}
                      </span>
                    </span>
                    <span role="radiogroup" aria-label={`${includer.name}: update or leave`} className="flex gap-1.5">
                      {(["update", "leave"] as const).map((action) => (
                        <ChoiceRow
                          key={action}
                          single
                          name={`includer-${includer.bundle_id}`}
                          selected={answers[includer.bundle_id] === action}
                          onToggle={() => setAnswers((a) => ({ ...a, [includer.bundle_id]: action }))}
                          title={action === "update" ? "Update" : "Leave"}
                        />
                      ))}
                    </span>
                  </div>
                ))}
              </div>
              <FieldHint>
                Update puts {name} v{draft.next_version} into that bundle&rsquo;s next version; its people get it
                when that bundle publishes. Leave keeps it where it is; its page offers v{draft.next_version} for
                later.
              </FieldHint>
            </div>
          )}

          <div>
            <FieldLabel htmlFor="version-note">Note for the history (optional)</FieldLabel>
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
      onRehearse={async (acknowledgeScope) =>
        (
          await rehearse.mutateAsync({
            note,
            migrate: willMigrate,
            acknowledge_scope: acknowledgeScope,
            ...includerAnswers,
          })
        ).plan
      }
      // The approval, cited. An empty string is what a publish reaching nobody
      // sends, and the backend accepts it there for the same reason it issued
      // none: there was no subject to approve.
      onApply={async (planId) =>
        (await apply.mutateAsync({ note, migrate: willMigrate, plan_id: planId || undefined, ...includerAnswers }))
          .plan
      }
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

/** "+ Safety basics v3", "Safety basics v3 → v4", "− Safety basics". */
export function IncludeChangeLine({ change }: { change: IncludeChange }) {
  if (change.from_version === 0) {
    return (
      <>
        <span aria-label="Adds" className="w-3 font-semibold text-accent-text">
          +
        </span>
        <span>
          {change.child_name} v{change.to_version}
        </span>
      </>
    );
  }
  if (change.to_version === 0) {
    return (
      <>
        <span aria-label="Drops" className="w-3 font-semibold text-warn-text">
          &minus;
        </span>
        <span>{change.child_name}</span>
      </>
    );
  }
  return (
    <>
      <span aria-hidden className="w-3" />
      <span>
        {change.child_name} v{change.from_version} → v{change.to_version}
      </span>
    </>
  );
}

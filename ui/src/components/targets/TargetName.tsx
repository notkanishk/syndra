"use client";

import { useState } from "react";

import { SettingRow } from "@/components/targets/SettingRow";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { STORAGE_LABEL, isStorage } from "@/lib/nav";
import { describeFailure, sentence } from "@/lib/outcome";
import { useSetTargetName, type TargetSummary } from "@/lib/queries/useTargets";

/**
 * The longest name a system may have. The schema's CHECK (migration 000091)
 * decides it and the backend's refusal says it; `name-limit.test.ts` holds
 * all three to one number.
 */
export const NAME_MAX = 60;

/**
 * What the Name setting says, by the case the system is in: not read yet
 * (members do not see it at all), named here, or shown by its add-on's
 * product or by its id. Two halves: what members see, which the settings row
 * says under the name, and what the field does, which only the dialog says.
 */
function nameHintParts(registered: TargetSummary): [seen: string, field: string] {
  const { name, name_from: from, kind } = registered;
  if (!kind) {
    return [
      "Members don't see this system until Syndra has read it.",
      from === "operator" ? "Then they see this name." : `Until you name it, it is ${name}, from its id.`,
    ];
  }
  if (from === "operator") {
    // Storage folds into the fixed Network storage row: no menu row of its own.
    const where = isStorage(registered) ? `on their ${STORAGE_LABEL} page` : "on their menu and their pages";
    return [`Members see this ${where}.`, "Clear it to go back to the default name."];
  }
  return from === "product"
    ? [`Members see it as ${name}, the product's own name.`, "Name it here to tell two of the same make apart."]
    : [`Members see it as ${name}, from its id.`, "Name it here to give it a name people know."];
}

export function nameHint(registered: TargetSummary): string {
  return nameHintParts(registered).join(" ");
}

/**
 * The Name row of the system's settings card (D-1 Q4, owner 18 Nov 2026; part
 * 2 frame 6): the label, the name and what members see, Change name in the
 * seat, the same shape as Home folders and Choose folder. A name is set about
 * once in a system's life, so it is one deliberate step away, in the dialog.
 */
export function TargetNameRow({ registered }: { registered: TargetSummary }) {
  const [changing, setChanging] = useState(false);
  return (
    <>
      <SettingRow
        label="Name"
        action={
          <Button variant="outline" size="sm" onClick={() => setChanging(true)}>
            Change name
          </Button>
        }
      >
        <span className="flex flex-col gap-0.5">
          <span className="text-body text-ink">{registered.name}</span>
          <span className="text-small text-faint">{nameHintParts(registered)[0]}</span>
        </span>
      </SettingRow>
      {changing && <TargetNameDialog registered={registered} onClose={() => setChanging(false)} />}
    </>
  );
}

/**
 * What members see a system called (addon-contract 3.10a): the one write
 * place for its name. Every page reads the name the roster carries; nowhere
 * else sets it. Blank goes back to the default the backend picks (the
 * add-on's product, else the system's id), which is what a second system of
 * the same make is named until somebody tells the two apart here. It ends on
 * its result, with Close, as Choose folder's dialog does.
 */
export function TargetNameDialog({ registered, onClose }: { registered: TargetSummary; onClose: () => void }) {
  const save = useSetTargetName(registered.target);
  const [draft, setDraft] = useState(registered.name_from === "operator" ? registered.name : "");
  const [done, setDone] = useState<string | null>(null);
  const trimmed = draft.trim();
  const named = registered.name_from === "operator";
  const unchanged = named ? trimmed === registered.name : trimmed === "";

  const submit = (name: string) => save.mutate(name, { onSuccess: (answer) => setDone(answer.name) });

  return (
    <Modal open onClose={onClose} size="sm" labelledBy="name-title" busy={save.isPending}>
      <ModalHeader title="What members see it called" titleId="name-title" />
      {/* One form around the field and the footer, so Enter in the field saves, as Save name does. */}
      <form
        className="contents"
        onSubmit={(event) => {
          event.preventDefault();
          if (!unchanged) submit(trimmed);
        }}
      >
        {done ? (
          <div className="px-6">
            <ActionOutcome
              word="Named"
              outcome={{ kind: "applied", message: `Members see it as ${done}.`, detail: "Nobody's access changed." }}
            />
          </div>
        ) : (
          <div className="flex flex-col gap-1.5 px-6">
            <FieldLabel htmlFor={`name-${registered.target}`}>Name</FieldLabel>
            <Input
              id={`name-${registered.target}`}
              value={draft}
              maxLength={NAME_MAX}
              placeholder={named ? undefined : registered.name}
              onChange={(event) => setDraft(event.target.value)}
            />
            <FieldHint>{nameHint(registered)}</FieldHint>
            {save.error && <p role="alert" className="text-small text-danger-text">{sentence(describeFailure(save.error))}</p>}
          </div>
        )}
        <ModalFooter cancel={done ? undefined : { onClick: onClose, disabled: save.isPending }}>
          {done ? (
            <Button variant="outline" onClick={onClose}>
              Close
            </Button>
          ) : (
            <>
              {named && (
                <Button variant="outline" isPending={save.isPending} onClick={() => submit("")}>
                  Use the default name
                </Button>
              )}
              <Button
                type="submit"
                variant="accent"
                isPending={save.isPending}
                disabled={unchanged}
                reason={unchanged ? "Type a new name first." : undefined}
              >
                Save name
              </Button>
            </>
          )}
        </ModalFooter>
      </form>
    </Modal>
  );
}

"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
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
 * product or by its id.
 */
export function nameHint(registered: TargetSummary): string {
  const { name, name_from: from, kind } = registered;
  if (!kind) {
    return from === "operator"
      ? "Members don't see this system until Syndra has read it. Then they see this name."
      : `Members don't see this system until Syndra has read it. Until you name it, it is ${name}, from its id.`;
  }
  if (from === "operator") return "Members see this on their menu and their pages. Clear it to go back to the default name.";
  return from === "product"
    ? `Members see it as ${name}, the product's own name. Name it here to tell two of the same make apart.`
    : `Members see it as ${name}, from its id. Name it here to give it a name people know.`;
}

/**
 * What members see a system called (addon-contract 3.10a): the one write
 * place for its name. Every page reads the name the roster carries; nowhere
 * else sets it. Blank goes back to the default the backend picks (the
 * add-on's product, else the system's id), which is what a second system of
 * the same make is named until somebody tells the two apart here.
 */
export function TargetName({ registered }: { registered: TargetSummary }) {
  const save = useSetTargetName(registered.target);
  const [draft, setDraft] = useState(registered.name_from === "operator" ? registered.name : "");
  const [done, setDone] = useState<string | null>(null);
  const trimmed = draft.trim();
  const named = registered.name_from === "operator";
  const unchanged = named ? trimmed === registered.name : trimmed === "";

  const submit = (name: string) =>
    save.mutate(name, {
      onSuccess: (answer) => {
        setDraft(answer.name_from === "operator" ? answer.name : "");
        setDone(answer.name);
      },
    });

  return (
    <Card>
      <CardHeader title="What members see it called" />
      <form
        className="flex flex-col gap-3 px-5 pb-5 pt-1"
        onSubmit={(event) => {
          event.preventDefault();
          if (!unchanged) submit(trimmed);
        }}
      >
        <div className="flex flex-col">
          <FieldLabel htmlFor={`name-${registered.target}`}>Name</FieldLabel>
          <Input
            id={`name-${registered.target}`}
            value={draft}
            maxLength={NAME_MAX}
            placeholder={named ? undefined : registered.name}
            onChange={(event) => {
              setDraft(event.target.value);
              setDone(null);
            }}
          />
          <FieldHint>{nameHint(registered)}</FieldHint>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" variant="accent" isPending={save.isPending} disabled={unchanged} reason={unchanged ? "Type a new name first." : undefined}>
            Save name
          </Button>
          {named && (
            <Button isPending={save.isPending} onClick={() => submit("")}>
              Use the default name
            </Button>
          )}
        </div>
        {done && (
          <ActionOutcome
            placement="inline"
            word="Named"
            outcome={{ kind: "applied", message: `Members see it as ${done}.`, detail: "Nobody's access changed." }}
          />
        )}
        {save.error && <p role="alert" className="text-[13.5px] text-danger-text">{sentence(describeFailure(save.error))}</p>}
      </form>
    </Card>
  );
}

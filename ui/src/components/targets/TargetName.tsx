"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { describeFailure, sentence } from "@/lib/outcome";
import { useSetTargetName, type TargetSummary } from "@/lib/queries/useTargets";

/**
 * What members see a system called (addon-contract 3.10a): the one write
 * place for its name. Every page reads the name the roster carries; nowhere
 * else sets it. Blank goes back to the default the backend picks (the
 * add-on's product, else the system's id), which is what a second system of
 * the same make is named until somebody tells the two apart here.
 */
export function TargetName({ registered }: { registered: TargetSummary }) {
  const save = useSetTargetName(registered.target);
  const [draft, setDraft] = useState(registered.name_set ? registered.name : "");
  const [done, setDone] = useState<string | null>(null);
  const trimmed = draft.trim();
  const unchanged = registered.name_set ? trimmed === registered.name : trimmed === "";

  const submit = (name: string) =>
    save.mutate(name, {
      onSuccess: (answer) => {
        setDraft(answer.name_set ? answer.name : "");
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
            maxLength={60}
            placeholder={registered.name_set ? undefined : registered.name}
            onChange={(event) => {
              setDraft(event.target.value);
              setDone(null);
            }}
          />
          <FieldHint>
            {registered.name_set
              ? "Members see this on their menu and their pages. Clear it to go back to the name the system gives itself."
              : `Members see it as ${registered.name}, the name the system gives itself. Name it here to tell two of the same make apart.`}
          </FieldHint>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" variant="accent" isPending={save.isPending} disabled={unchanged} reason={unchanged ? "Type a new name first." : undefined}>
            Save name
          </Button>
          {registered.name_set && (
            <Button isPending={save.isPending} onClick={() => submit("")}>
              Use the system&apos;s own name
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

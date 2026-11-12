"use client";

import { useState } from "react";

import * as copy from "@/components/usage/copy";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { describeFailure, sentence } from "@/lib/outcome";
import { useSetStatsRetention, useTargetStats } from "@/lib/queries/useStats";

/** The ceiling the backend keeps per-person numbers to; "Keep the full 400 days" is back to it. */
export const RETENTION_MAX = 400;

/**
 * How long a system keeps per-person numbers: the one write place for it (the
 * overview's totals only say what is set, here). Shortening deletes history
 * for good within a day (the retention runs at start-up and every 24 hours
 * after), so it asks first, in a dialog; lengthening, or going back to the
 * full 400 days, changes nothing that exists and does not. A number the
 * backend would refuse (below 1) is sent without asking: its 422 is the
 * only answer, never a question about deleting.
 */
export function StatsRetention({ target }: { target: string }) {
  const stats = useTargetStats(target);
  // Nothing to set where the add-on reports no statistics, or Syndra has not read the page yet.
  if (!stats.data?.usage.reported) return null;
  return <RetentionForm target={target} days={stats.data.retention.subject_days} set={stats.data.retention.set} />;
}

function RetentionForm({ target, days, set }: { target: string; days: number; set: boolean }) {
  const save = useSetStatsRetention(target);
  const [draft, setDraft] = useState(String(days));
  const [asking, setAsking] = useState<number | null>(null);
  const [done, setDone] = useState<number | null>(null);
  const wanted = Number(draft);
  const valid = draft.trim() !== "" && Number.isInteger(wanted);
  const unchanged = !valid || wanted === days;

  const submit = (value: number | null) =>
    save.mutate(value, {
      onSuccess: (answer) => {
        setDraft(String(answer.subject_days));
        setDone(answer.subject_days);
        setAsking(null);
      },
      onError: () => setAsking(null),
    });

  return (
    <Card>
      <CardHeader title={copy.RETENTION_TITLE} />
      {/* noValidate: min and max steer the spinner, and the backend alone refuses a number it does not keep (its words are shown). */}
      <form
        noValidate
        className="flex flex-col gap-3 px-5 pb-5 pt-1"
        onSubmit={(event) => {
          event.preventDefault();
          if (unchanged) return;
          if (1 <= wanted && wanted < days) setAsking(wanted);
          else submit(wanted);
        }}
      >
        <div className="flex flex-col">
          <FieldLabel htmlFor={`retention-${target}`}>{copy.RETENTION_FIELD}</FieldLabel>
          <Input
            id={`retention-${target}`}
            type="number"
            inputMode="numeric"
            min={1}
            max={RETENTION_MAX}
            step={1}
            value={draft}
            className="max-w-[140px]"
            onChange={(event) => {
              setDraft(event.target.value);
              setDone(null);
              save.reset(); // an old refusal is not about the number now typed
            }}
          />
          <FieldHint>{copy.retentionHint(days, set)}</FieldHint>
        </div>
        <div className="flex flex-wrap items-start gap-3">
          <Button type="submit" variant="accent" isPending={save.isPending} disabled={unchanged} reason={unchanged ? copy.RETENTION_UNCHANGED : undefined}>
            {copy.RETENTION_SAVE}
          </Button>
          {set && (
            <Button isPending={save.isPending} onClick={() => submit(null)}>
              {copy.RETENTION_FULL}
            </Button>
          )}
        </div>
        {asking !== null && (
          <Modal open onClose={() => !save.isPending && setAsking(null)} busy={save.isPending} size="sm">
            <ModalHeader title={copy.retentionAskTitle(asking)} lede={copy.retentionAsk(asking)} />
            <ModalFooter cancel={{ onClick: () => setAsking(null), label: copy.RETENTION_ASK_NO, disabled: save.isPending }}>
              {/* dangerConfirm: the numbers older than this are deleted for good. */}
              <Button variant="dangerConfirm" isPending={save.isPending} onClick={() => submit(asking)}>
                {copy.retentionAskYes(asking)}
              </Button>
            </ModalFooter>
          </Modal>
        )}
        {done !== null && (
          <ActionOutcome
            placement="inline"
            word={copy.RETENTION_WORD}
            outcome={{ kind: "applied", message: copy.retentionSaved(done), detail: copy.RETENTION_SAVED_NEXT }}
          />
        )}
        {save.error && <p role="alert" className="text-[13.5px] text-danger-text">{sentence(describeFailure(save.error))}</p>}
      </form>
    </Card>
  );
}

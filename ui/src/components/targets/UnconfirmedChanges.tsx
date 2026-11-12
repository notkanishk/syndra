"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";
import { personOr } from "@/lib/names";

import { useQueryClient } from "@tanstack/react-query";

import { invalidateAfter } from "@/lib/queries/invalidation";
import { useState } from "react";

import { checkoutWording } from "@/components/actions/copy";
import { UserName } from "@/components/names";
import { useAccountHref } from "@/components/system/ServiceAccountTag";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { LeavingRow, useLeavingRows } from "@/lib/motion";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { describeFailure, sentence } from "@/lib/outcome";
import { formatAt, formatRelative } from "@/lib/format";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { homeSettingWording } from "@/components/kinds/storage/unconfirmed";
import { useAnswerUnconfirmed, type UnconfirmedOperation } from "@/lib/queries/useTargets";

/**
 * Changes a connected system never answered (A-09 §3–4, A-13 §5), first in
 * Waiting on you, newest first. Each may or may not have happened, and only a
 * person can say: one verb, Say what happened, asks them to look first and
 * then records what they found. Nothing is sent to the system.
 *
 * Passwords never reach here: a member settles their own by setting it again,
 * and a rotate belongs to its revoke. What is left in practice is deletes.
 */
export function UnconfirmedChanges({
  target,
  operations,
  ready,
}: {
  target: string;
  operations: UnconfirmedOperation[];
  ready: boolean;
}) {
  const leaving = useLeavingRows(operations, (op) => op.id, ready);
  const [open, setOpen] = useState<UnconfirmedOperation | null>(null);
  const client = useQueryClient();

  return (
    <>
      {/* No rows: nothing shows, no card, no zero (A-09 §3). */}
      {leaving.rows.length > 0 && (
        <Card>
          {leaving.rows.map((op, index) => (
            <LeavingRow key={op.id} {...leaving.props(op.id)}>
              <UnconfirmedRow target={target} op={op} first={index === 0} onSay={() => setOpen(op)} />
            </LeavingRow>
          ))}
        </Card>
      )}
      {open && (
        <SayWhatHappened
          target={target}
          op={open}
          onClose={(answered) => {
            setOpen(null);
            // The row leaves on the next read, after the result was seen (A-05 §2.6).
            if (answered) {
              invalidateAfter(client, "target", { target });
            }
          }}
        />
      )}
    </>
  );
}

const SYNDRA = /^system(:|$)/;

/** A name as text, for a sentence that is a string (a title, an aria-label). */
function usePersonName(id: string): string {
  return personOr(useNameResolver().resolveUser(id).value?.display_name);
}

/** Each kind, worded (A-09 §4's table, less the two password rows A-13 §5 drops). */
function wording(op: UnconfirmedOperation, who: string, name: string) {
  const account = op.about || `${who}'s account`;
  switch (op.operation) {
    case "account.purge":
      return {
        asked: [`Delete `, `'s account and files on ${name}`],
        question: `Did ${name} delete ${who}'s account and files?`,
        tell: `Read ${name} again. If ${account} is gone from Accounts on ${name}, it happened. Syndra won't send a delete again by itself.`,
        yes: `${account} and its files are gone from ${name}.`,
        no: `${account} is still on ${name}, with its files.`,
        done: `${name} deleted ${who}'s account and files.`,
        notDone: `You recorded that ${name} didn't delete ${who}'s account and files.`,
      };
    // Syndra's lists of accounts change only when the system confirms an
    // assign or a stop, and an answer here records the answer alone (it writes
    // no binding). So the lists cannot tell, and the next step is to do it
    // again: the system answers either way, and the lists follow (6.3r).
    case "account.adopt":
      return {
        asked: [`Assign ${account} to `, ""],
        question: `Did ${name} assign ${account} to ${who}?`,
        tell: `Syndra's lists change only when ${name} confirms, so they can't tell you. Assign ${account} to ${who} again: ${name} answers either way, and Syndra lists it then.`,
        yes: `${name} assigned ${account} to ${who}. Syndra lists it once you assign it again.`,
        no: `${name} didn't assign ${account}.`,
        done: `You recorded that ${name} assigned ${account} to ${who}. Assign it again to have Syndra list it.`,
        notDone: `You recorded that ${name} didn't assign ${account} to ${who}.`,
      };
    case "account.release":
      return {
        asked: [`Let go of `, `'s account on ${name}, keeping it`],
        question: `Did ${name} let go of ${who}'s account?`,
        tell: `Syndra's lists change only when ${name} confirms, so they can't tell you. Stop tracking ${account} again: ${name} answers either way, and Syndra stops listing it then.`,
        yes: `${name} let go of ${account}. Syndra stops listing it once you stop tracking it again.`,
        no: `${name} still holds ${account} for ${who}.`,
        done: `You recorded that ${name} let go of ${account}. Stop tracking it again to have Syndra stop listing it.`,
        notDone: `You recorded that ${name} didn't let go of ${account}.`,
      };
    // A maintenance change is about the system itself, not a person
    // (addons/read.go SetLifecycle: subject is the target, about is the state).
    case "lifecycle.set": {
      const state = op.about === "draining" ? "finishing up" : op.about === "read_only" ? "read-only" : op.about ?? "a new state";
      return {
        asked: [`Set ${name} to ${state}`, null],
        question: `Did ${name} change to ${state}?`,
        tell: `The state at the top of this page shows what ${name} says it is now.`,
        yes: `${name} is ${state}.`,
        no: `${name} is still in the state it was.`,
        done: `${name} is ${state}.`,
        notDone: `You recorded that ${name} didn't change to ${state}.`,
      };
    }
    // A physical act is settled at the desk, so the row says to ask there (addon-contract 6c).
    case "item.checkout":
      return checkoutWording(op.about, who, name);
    case "setting.put":
      return homeSettingWording(op, name);
    default:
      return {
        asked: [`A change for `, ` on ${name}`],
        question: `Did ${name} make this change for ${who}?`,
        tell: `Look at ${name}: what it holds now says whether it happened.`,
        yes: `${name} has the change.`,
        no: `${name} doesn't have the change.`,
        done: `${name} has the change for ${who}.`,
        notDone: `You recorded that ${name} didn't make the change for ${who}.`,
      };
  }
}

function UnconfirmedRow({
  target,
  op,
  first,
  onSay,
}: {
  target: string;
  op: UnconfirmedOperation;
  first: boolean;
  onSay: () => void;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const who = usePersonName(op.subject_id);
  const w = wording(op, who, name);
  const [pre, post] = w.asked;
  const today = new Date(op.sent_at).toDateString() === new Date().toDateString();
  const accountHref = useAccountHref();

  return (
    <div className={`${first ? "" : "row-divider "}flex flex-wrap items-start gap-x-[18px] gap-y-2.5 px-5 py-4`}>
      <span className="flex min-w-0 flex-[1_1_380px] flex-col gap-1">
        <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <span className="text-[15px] font-semibold [text-wrap:pretty]">
            {pre}
            {post !== null && (
              <>
                <UserName id={op.subject_id} href={accountHref(op.subject_id)} className="text-ink" />
                {post}
              </>
            )}
          </span>
          <StatusSeat tone="warn" word="Not confirmed" />
        </span>
        <span className="text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
          Sent to {name} {formatAt(op.sent_at)}
          {today ? `, ${formatRelative(op.sent_at)}` : ""}. No answer came back, so it may or may not have
          happened.
        </span>
        <span className="text-[12.5px] text-faint">
          {SYNDRA.test(op.actor_id) ? (
            "Asked by Syndra, bringing accounts in line"
          ) : (
            <>
              Asked by <UserName id={op.actor_id} />
            </>
          )}
        </span>
      </span>
      <span className="ml-auto inline-flex flex-none whitespace-nowrap">
        <Button variant="outline" size="sm" onClick={onSay} aria-label={`Say what happened: ${w.question}`}>
          Say what happened
        </Button>
      </span>
    </div>
  );
}

function SayWhatHappened({
  target,
  op,
  onClose,
}: {
  target: string;
  op: UnconfirmedOperation;
  onClose: (answered: boolean) => void;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const who = usePersonName(op.subject_id);
  const asker = usePersonName(op.actor_id);
  const w = wording(op, who, name);
  const answer = useAnswerUnconfirmed(target);
  const [happened, setHappened] = useState<boolean | null>(null);
  const result = answer.isSuccess ? (answer.variables?.happened ?? null) : null;
  const asked = SYNDRA.test(op.actor_id) ? "by Syndra itself" : `by ${asker}`;

  return (
    <Modal
      open
      onClose={() => onClose(answer.isSuccess)}
      busy={answer.isPending}
      size="md"
      labelledBy="settle-title"
    >
      <ModalHeader
        titleId="settle-title"
        title={w.question}
        lede={`Syndra sent it ${formatAt(op.sent_at)}, asked ${asked}, and no answer came back. Until you say, Syndra treats it as neither done nor failed.`}
      />
      <div className="flex flex-col gap-4 px-6">
        {result === null ? (
          <>
            <div className="flex flex-col gap-1.5">
              <span className="type-label text-label">How to tell</span>
              <p className="text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">{w.tell}</p>
            </div>
            <div role="radiogroup" aria-label="What happened" className="flex flex-col gap-1.5">
              <ChoiceRow
                single
                name="what-happened"
                outlined
                selected={happened === true}
                onToggle={() => setHappened(true)}
                title="It happened"
                description={w.yes}
              />
              <ChoiceRow
                single
                name="what-happened"
                outlined
                selected={happened === false}
                onToggle={() => setHappened(false)}
                title="It didn't happen"
                description={w.no}
              />
            </div>
            {answer.error && (
              <p role="alert" className="text-[13.5px] text-danger-text">
                {sentence(describeFailure(answer.error))}
              </p>
            )}
          </>
        ) : (
          // Done lifts and blooms once; Not done appears still (S6).
          <ActionOutcome
            word={result ? "Done" : "Not done"}
            outcome={{
              kind: result ? "applied" : "no_change",
              message: result ? w.done : w.notDone,
              detail: "It leaves Waiting on you. Audit keeps your answer, with your name.",
            }}
          />
        )}
      </div>
      <ModalFooter
        cancel={result === null ? { onClick: () => onClose(false), disabled: answer.isPending } : undefined}
        note={result === null ? `Nothing is sent to ${name}. This records what you found, with your name.` : undefined}
      >
        {result === null ? (
          <Button
            variant="accent"
            disabled={happened === null}
            isPending={answer.isPending}
            onClick={() => happened !== null && answer.mutate({ id: op.id, happened })}
          >
            {happened === null ? "Record what happened" : happened ? "Record it as done" : "Record it as not done"}
          </Button>
        ) : (
          <Button variant="outline" onClick={() => onClose(true)}>
            Close
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

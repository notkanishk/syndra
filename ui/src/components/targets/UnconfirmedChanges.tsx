"use client";

import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { UserName } from "@/components/names";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { describeFailure, sentence } from "@/lib/outcome";
import { formatAt, formatRelative } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useNameResolver } from "@/lib/queries/useNameResolver";
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
              client.invalidateQueries({ queryKey: ["targets", target, "unconfirmed"] });
              client.invalidateQueries({ queryKey: ["governance", "indicators"] });
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
  return useNameResolver().resolveUser(id).value?.display_name?.trim() || "this person";
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
    case "account.adopt":
      return {
        asked: [`Assign ${account} to `, ""],
        question: `Did ${name} assign ${account} to ${who}?`,
        tell: `If ${account} is under Accounts Syndra manages, it happened.`,
        yes: `${account} is ${who}'s account, and Syndra manages it.`,
        no: `${account} is still under Not made by Syndra.`,
        done: `${account} is ${who}'s account on ${name}.`,
        notDone: `You recorded that ${name} didn't assign ${account} to ${who}.`,
      };
    case "account.release":
      return {
        asked: [`Let go of `, `'s account on ${name}, keeping it`],
        question: `Did ${name} let go of ${who}'s account?`,
        tell: `If ${account} is under Not made by Syndra, it happened.`,
        yes: `${account} stays on ${name}, and Syndra no longer manages it.`,
        no: `Syndra still manages ${account}.`,
        done: `Syndra no longer manages ${account}. It stays on ${name}.`,
        notDone: `You recorded that ${name} didn't let go of ${account}.`,
      };
    case "setting.put":
      return {
        asked: [`Change where home folders go to ${op.about ?? "a new place"}`, null],
        question: `Did ${name} change where home folders go?`,
        tell: `Home folders, in the setup card above, shows what ${name} holds now.`,
        yes: `New home folders go to ${op.about ?? "the new place"}.`,
        no: `Home folders still go where they did.`,
        done: `New home folders on ${name} go to ${op.about ?? "the new place"}.`,
        notDone: `You recorded that ${name} didn't change where home folders go.`,
      };
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
  const name = targetLabel(target);
  const who = usePersonName(op.subject_id);
  const w = wording(op, who, name);
  const [pre, post] = w.asked;
  const today = new Date(op.sent_at).toDateString() === new Date().toDateString();

  return (
    <div className={`${first ? "" : "row-divider "}flex flex-wrap items-start gap-x-[18px] gap-y-2.5 px-5 py-4`}>
      <span className="flex min-w-0 flex-[1_1_380px] flex-col gap-1">
        <span className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <span className="text-[15px] font-semibold [text-wrap:pretty]">
            {pre}
            {post !== null && (
              <>
                <Link href={`/users/${encodeURIComponent(op.subject_id)}`} className="text-ink link-hover">
                  <UserName id={op.subject_id} />
                </Link>
                {post}
              </>
            )}
          </span>
          <span className="inline-flex items-center gap-[7px] whitespace-nowrap text-[13.5px] font-semibold text-warn-text">
            <StatusDot tone="warn" />
            Not confirmed
          </span>
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
  const name = targetLabel(target);
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
          <div role="status" className="flex flex-col gap-1.5 pb-2.5 pt-1.5">
            <span className={`inline-flex items-center gap-2.5 font-display text-[22px] font-semibold ${result ? "lift-in" : ""}`}>
              {/* Done lifts and blooms once; Not done appears still (S6). */}
              <StatusDot tone={result ? "healthy" : "neutral"} bloom={result} />
              {result ? "Done" : "Not done"}
            </span>
            <p className="text-[15px] leading-[1.55] [text-wrap:pretty]">{result ? w.done : w.notDone}</p>
            <span className="text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
              It leaves Waiting on you. Audit keeps your answer, with your name.
            </span>
          </div>
        )}
      </div>
      <ModalFooter
        note={result === null ? `Nothing is sent to ${name}. This records what you found, with your name.` : undefined}
      >
        {result === null ? (
          <>
            <span className="inline-flex flex-none whitespace-nowrap">
              <Button variant="outline" onClick={() => onClose(false)} disabled={answer.isPending}>
                Cancel
              </Button>
            </span>
            <span className="inline-flex flex-none whitespace-nowrap">
              <Button
                variant="accent"
                disabled={happened === null}
                isPending={answer.isPending}
                onClick={() => happened !== null && answer.mutate({ id: op.id, happened })}
              >
                {happened === null ? "Record what happened" : happened ? "Record it as done" : "Record it as not done"}
              </Button>
            </span>
          </>
        ) : (
          <Button variant="outline" onClick={() => onClose(true)}>
            Close
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

import type { UnconfirmedOperation } from "@/lib/queries/useTargets";

/**
 * How an unconfirmed `setting.put` is worded (A-09 §4): the one setting a
 * storage system has is where home folders go. Storage words, so they live in
 * the storage kind; the table of unconfirmed changes reads this for that row.
 */
export function homeSettingWording(op: UnconfirmedOperation, name: string) {
  return {
    asked: [`Change where home folders go to ${op.about ?? "a new place"}`, null],
    question: `Did ${name} change where home folders go?`,
    tell: `Home folders, in the setup card above, shows what ${name} holds now.`,
    yes: `New home folders go to ${op.about ?? "the new place"}.`,
    no: `Home folders still go where they did.`,
    done: `New home folders on ${name} go to ${op.about ?? "the new place"}.`,
    notDone: `You recorded that ${name} didn't change where home folders go.`,
  };
}

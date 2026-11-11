import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * Every sentence the generic read renderers say, in one place, so the copy
 * guide (`plain-language-copy/design.md` section 12) and a test can hold them
 * to the same words. Each outcome ends on a next step or "Nothing for you to do."
 */

/** A readable list with nothing in it. */
export const emptyRecords = (name: string) => `You hold nothing from ${name}. ${NOTHING_TO_DO}`;
export const emptyMeters = (name: string) => `${name} reports no usage. ${NOTHING_TO_DO}`;
export const emptyEvents = (name: string) => `No recorded activity on ${name}. ${NOTHING_TO_DO}`;

/** The read did not come back: not a claim that the answer is "none". */
export const NOT_READABLE_NEXT = "Try again in a minute. If it keeps failing, ask makerspace staff.";
export const notReadableTitle = (name: string) => `${name} could not be read`;
export const notReadableBody = (name: string) => `Syndra asked ${name} and did not get a usable answer. This is not a claim that nothing is there.`;

/** Usage the add-on could not read, whole or in part. */
export const usageUnread = (name: string, some: boolean) => (some ? `Some usage could not be read from ${name}.` : `Usage could not be read from ${name}.`);

export const NO_LIMIT = "no limit";
export const ALSO_SAYS = "What the add-on also says";
export const noMoreLines = (n: number) => `…and ${n} more`;

/** Places that were not recording this person: a quiet list is not proof. */
export const uncoveredNote = (places: string, name: string) =>
  `${places} were not recording ${name}, so nothing from them appears here, whether or not it happened. Switch recording on there to see them.`;

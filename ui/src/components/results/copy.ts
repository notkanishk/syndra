import { formatList } from "@/lib/format";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * Every sentence the generic read renderers say, in one place, so the copy
 * guide (`plain-language-copy/design.md` sections 12 and 13) and a test can hold
 * them to the same words. Each outcome ends on a next step or "Nothing for you to do."
 * A sentence that differs by who is reading takes the reader.
 */

/** Who is reading: a member about themselves, or an operator about `person`. */
export type Reader = { audience: "member" } | { audience: "operator"; person: string };
export const MEMBER: Reader = { audience: "member" };

/** A readable list with nothing in it. "Records" means things held or done, so the sentence fits any shape. */
export const emptyRecords = (name: string, who: Reader) =>
  `${name} lists nothing for ${who.audience === "member" ? "you" : who.person}. ${NOTHING_TO_DO}`;
export const emptyMeters = (name: string) => `${name} reports no usage. ${NOTHING_TO_DO}`;
export const emptyEvents = (name: string) => `No recorded activity on ${name}. ${NOTHING_TO_DO}`;

/** The read did not come back: not a claim that the answer is "none". The title is per read, so it never contradicts the system's own "read just now". */
export const notReadableTitle = (name: string) => `One read from ${name} did not come back`;
export const notReadableBody = (name: string) => `Syndra asked ${name} and did not get a usable answer. This is not a claim that nothing is there.`;
export const notReadableNext = (who: Reader) =>
  who.audience === "member"
    ? "Try again in a minute. If it keeps failing, ask makerspace staff."
    : "Try again in a minute. If it keeps failing, check the add-on's health on its system page.";

/** Usage the add-on could not read, whole or in part. */
export const usageUnread = (name: string, some: boolean) => (some ? `Some usage could not be read from ${name}.` : `Usage could not be read from ${name}.`);
export const usageUnreadBody = (some: boolean) => (some ? "The usage above is what was read." : "This is not a claim that nothing is used.");

export const NO_LIMIT = "no limit";
export const ofLimit = (limit: string) => `of ${limit}`;
export const sinceDate = (date: string) => `Since ${date}`;
export const dueDate = (date: string) => `Due ${date}`;
/** The records list's column headings: each sorts the list (the default is Due, soonest first). */
export const RECORD_HEADS = { record: "Record", since: "Since", due: "Due" } as const;
/** What a screen reader hears after "Sort by" on each of those headings. */
export const RECORD_SORT_BY = { record: "name", since: "date held", due: "due date" } as const;
export const ALSO_SAYS = "What the add-on also says";
export const noMoreLines = (n: number) => `…and ${n} more`;

/**
 * Places that were not recording the person: a quiet list is not proof. The
 * person is named ("you" for a member), one place or many; an operator is told
 * what to do, a member that there is nothing for them to do.
 */
export const uncoveredNote = (places: string[], who: Reader) => {
  const whom = who.audience === "member" ? "you" : who.person;
  const them = places.length === 1 ? "that place" : "those places";
  return `${formatList(places)} did not record ${whom}, so nothing from ${them} appears here, whether or not it happened. ${
    who.audience === "member" ? NOTHING_TO_DO : "Switch recording on there."
  }`;
};

/** Under an empty activity list: an empty list that is not the same as nothing having happened. */
export const UNCOVERED_GUIDANCE = "Some places were not recording this account, so this is not the same as nothing having happened. See below.";

/**
 * How fresh a read is (copy guide section 13). The first is a copy the add-on
 * kept, the next a "Read again" pressed inside its cooldown, the last a system
 * that cannot be reached; each draws beside the control that is its next step,
 * and the last one says its own. The words on the control are fixed.
 */
export const READ_AGAIN = "Read again";
export const READING = "Reading";
export const copyLine = (name: string, age: string) => `${name} was read ${age}`;
export const cooldownLine = (name: string, seconds: number) => `${name} was read moments ago; you can read again in ${seconds} s`;
/** "Check now" or a removal the cooldown refused, going again by itself once the count ends (useRetryOnce); STOP_WAITING stops it. */
export const retryingLine = (name: string, seconds: number) => `${name} was read moments ago; trying again by itself in ${seconds} s`;
export const STOP_WAITING = "Stop waiting";
/** "Check now"'s step once a skipped pass is not going again by itself: while its count runs, and after. */
export const checkAgainNext = (seconds: number) =>
  seconds > 0 ? "Press Check now again when the count ends." : "Press Check now again.";
export const unreachableLine = (name: string, time: string, who: Reader) =>
  `${name} can't be reached right now; this is what it said at ${time}. ${notReadableNext(who)}`;

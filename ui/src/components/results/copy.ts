import { formatList } from "@/lib/format";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { REASON, type Why } from "@/lib/queries/read";

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

/** The next step beside a read that did not come back, by who is reading. */
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
/** A read that hit its cap, in the same words wherever it is said: the strip's clause and the reconcile line's. */
export const FIRST_PART_READ = "only the first part was read";
/** "Check now"'s step once a skipped pass is not going again by itself: while its count runs, and after. */
export const checkAgainNext = (seconds: number) =>
  seconds > 0 ? "Press Check now again when the count ends." : "Press Check now again.";
export const unreachableLine = (name: string, time: string, who: Reader) =>
  `${name} can't be reached right now; this is what it said at ${time}. ${notReadableNext(who)}`;

/**
 * Why a system was not read (addon-contract design §13.1; copy guide section 19),
 * ending in its next step. A member reads one sentence for both outages, since
 * which box is down is not theirs to fix; an operator is told which side is
 * down and what to check, and the add-on's own words come first where it is the
 * one that knows (its app down, its refusal). The backend writes the same
 * sentences into a 503's `message`, which is said as it came; both ends are
 * held to the guide's table by a test each.
 */
export function whyLine(why: Why, system: string, who: Reader, product = system): string {
  if (why.message) return why.message;
  const { reason, cause, detail, retryAfter } = why;
  if (reason === REASON.cooldown) return `${system} was read moments ago. Read it again in ${Math.max(1, retryAfter ?? 1)} s.`;
  if (who.audience === "member") {
    return reason === REASON.addonUnreachable || reason === REASON.appUnreachable
      ? `${system} can't be read right now. ${NOTHING_TO_DO}`
      : `${system} can't be read right now. Ask makerspace staff.`;
  }
  const said = detail?.trim();
  if (said && (reason === REASON.appUnreachable || reason === REASON.readRefused)) return said.endsWith(".") ? said : `${said}.`;
  switch (reason) {
    case REASON.addonUnreachable: {
      const again = retryAfter && retryAfter > 0 ? ` Syndra tries again in ${retryAfter} s.` : "";
      switch (cause) {
        case "wrong_host":
          return `Something answered at ${system}'s add-on address, but it isn't the add-on registered here. Check the address and the shared secret, then restart the add-on.${again}`;
        case "timeout":
          return `${system}'s add-on started answering and stopped. Syndra keeps trying. If it lasts, read the add-on's log.${again}`;
        case "fault":
          return `${system}'s add-on answered with an error it didn't explain. Read the add-on's log.${again}`;
        case "no_key":
          return `Syndra can't load ${system}'s shared secret. Check Syndra's secret file.${again}`;
        default:
          return `Syndra can't reach ${system}'s add-on. Check that its container is running. Syndra keeps trying.${again}`;
      }
    }
    case REASON.appUnreachable:
      return `${system}'s add-on is running but can't reach ${product}. Check that ${product} is on.`;
    case REASON.addonOutdated:
      return `${system}'s add-on doesn't offer this. Update the add-on.`;
  }
  return `${system}'s add-on didn't take this. Read the add-on's log.`;
}

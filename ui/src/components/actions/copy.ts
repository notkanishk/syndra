import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * Every sentence a member's physical action says, in one place, so the copy
 * guide (`plain-language-copy/design.md` section 14) and a test hold them to
 * the same words. Each outcome ends on a next step or "Nothing for you to do."
 * A failure that is not the member's is never worded as if it were: the system
 * did not answer, a record is not settled, someone else has it.
 */

/** What an action is called on its button and its dialog: the add-on's id, said in the product's words. */
const ACTION_LABEL: Record<string, string> = { "item.checkout": "Check out an item" };
const capitalise = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);
export const actionLabel = (id: string) => ACTION_LABEL[id] ?? capitalise(id.replace(/[._]/g, " "));

/** A parameter's name as words ("item_id" is "item id"). */
export const paramWords = (name: string) => name.replace(/_/g, " ");
export const paramLabel = (name: string) => capitalise(paramWords(name));

/** The form. */
export const FORM_LEDE = "Say which one. Nothing is sent until you confirm.";
export const TARGET_ID_HINT = "Type the id on its label, or scan it.";
export const PREVIEW_BUTTON = "Show what will happen";
export const CONFIRM_BUTTON = "Confirm";

/** The preview. */
export const PREVIEW_LEDE = "This is what will happen. Nothing has been sent yet.";
export const previewNote = (name: string) => `Nothing is sent to ${name} until you press Confirm.`;
export const PREVIEW_NO_CHANGE = "You already have this, so there is nothing to confirm. Nothing for you to do.";
export const PREVIEW_UNREADABLE_TITLE = "The preview could not be shown";
export const previewUnreadableBody = (name: string) => `${name} answered, but Syndra could not read what it said. Nothing was sent.`;
export const PREVIEW_UNREADABLE_NEXT = "Go back and try again.";

/** The action, while their last one is still being settled. */
export const UNSETTLED_REASON = "Your last one is still being checked. Wait for it before trying again.";

/** The camera. */
export const SCAN_BUTTON = "Scan with camera";
export const SCAN_ARIA = "Scan the label with your camera";
export const SCAN_TITLE = "Scan the label";
export const SCAN_LEDE = "Hold the label in front of the camera. The id is filled in when it reads it.";
export const SCAN_STARTING = "Starting the camera";
export const SCAN_DENIED = "The camera could not be used. Type the id instead.";

/** What happened: the word beside the dot, then the sentence, then the step. */
export const WORD = { done: "Done", noChange: "Nothing changed", notDone: "Not done", checking: "Checking", askDesk: "Not sure yet" } as const;

export const DONE = "It went through";
export const NO_CHANGE = "You already have this";
export const NOT_DONE = "It did not happen. Nothing was changed";
export const NOT_DONE_NEXT = "You can try again.";
export const CHECKING = "We're checking whether it went through";
export const CHECKING_DETAIL = "It was sent, and the answer did not come back.";
export const CHECKING_NEXT = `${NOTHING_TO_DO} This page checks again on its own, and Syndra will not send it again.`;
export const CHECKING_LATE = "We could not check just now. This page tries again on its own.";
export const ASK_DESK = "We still can't tell whether it went through";
export const ASK_DESK_DETAIL = "Staff have been told.";
export const ASK_DESK_NEXT = "Ask at the desk.";

const TRY_LATER = "Try again in a minute. If it keeps failing, ask makerspace staff.";

/**
 * A refusal, by its code (handlers/member_operations.go): what is true, and the
 * one step. `details` carries only names and numbers, never what was typed.
 * Undefined for a code the page has no words for.
 */
export function refusal(code: string, details: Record<string, string> = {}): { message: string; next: string } | undefined {
  const param = paramWords(details.param ?? "id");
  switch (code) {
    case "UNKNOWN_RESOURCE":
      return { message: `Nothing matches that ${param}`, next: "Check the label and type it again." };
    case "NOT_ENTITLED":
      return { message: `Your roles do not allow this for that ${param}`, next: "Ask makerspace staff, or ask for access in Requests." };
    case "RESOURCE_TAKEN":
      return { message: "Someone else has that one", next: "Pick another, or ask at the desk." };
    case "LIMIT_REACHED":
      return {
        message: details.limit && details.count && details.unit ? `You are at your limit: ${details.count} of ${details.limit} ${details.unit}` : "You are at your limit",
        next: "Return one first, then try again.",
      };
    case "CALL_UNSETTLED":
      return { message: "Your last one is still being checked", next: "Wait for it before trying again." };
    case "ACTION_REFUSED":
      return { message: "That system refused this and gave no reason", next: "Ask makerspace staff." };
    case "NO_ENTITLEMENT":
      return { message: "None of your roles reaches this system", next: "Ask for access in Requests." };
    case "ACCOUNT_PENDING":
      return { message: "Your account here is not made yet", next: `${NOTHING_TO_DO} Syndra makes it.` };
    case "TARGET_PAUSED":
      return { message: "Changes on that system are paused", next: "Try again later. If it stays paused, ask makerspace staff." };
    case "ACTION_UNAVAILABLE":
      return { message: "That system cannot do this right now", next: TRY_LATER };
    case "NO_SUCH_ACTION":
      return { message: "This system does not offer that", next: "Reload the page to see what it offers now." };
    case "TARGET_UNREACHABLE":
      return { message: "That system is not answering. Nothing went out", next: TRY_LATER };
    case "PREVIEW_UNANSWERED":
      return { message: "That system did not answer the preview. Nothing went out", next: TRY_LATER };
    case "CALL_UNSETTLED_UNKNOWN":
      return { message: "We could not check on your last one. Nothing went out", next: "Try again in a minute." };
    case "TOO_MANY_ATTEMPTS":
      return { message: "That was tried too many times in a short while", next: "Wait a few minutes, then try again." };
    default:
      return undefined;
  }
}

/** Every code `refusal` words, so a test can walk them. */
export const REFUSAL_CODES = [
  "UNKNOWN_RESOURCE",
  "NOT_ENTITLED",
  "RESOURCE_TAKEN",
  "LIMIT_REACHED",
  "CALL_UNSETTLED",
  "ACTION_REFUSED",
  "NO_ENTITLEMENT",
  "ACCOUNT_PENDING",
  "TARGET_PAUSED",
  "ACTION_UNAVAILABLE",
  "NO_SUCH_ACTION",
  "TARGET_UNREACHABLE",
  "PREVIEW_UNANSWERED",
  "CALL_UNSETTLED_UNKNOWN",
  "TOO_MANY_ATTEMPTS",
] as const;

/**
 * The operator's row for a checkout whose answer was lost (Waiting on you):
 * what was asked, how to tell, and each answer. A physical act is settled by
 * asking at the desk, never by reading a system.
 */
export function checkoutWording(item: string | undefined, who: string, name: string) {
  const what = item ?? "an item";
  return {
    asked: [`Check out ${what} to `, ""] as [string, string],
    question: `Did ${what} go out to ${who}?`,
    tell: `Ask at the desk whether ${what} went out to ${who}. If it did, it happened.`,
    yes: `${what} went out to ${who}.`,
    no: `${what} did not go out to ${who}.`,
    done: `${what} went out to ${who} on ${name}.`,
    notDone: `You recorded that ${what} did not go out to ${who}.`,
  };
}

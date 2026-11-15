import { formatClock } from "@/lib/format";
import { sentence } from "@/lib/outcome";

/**
 * Why a change waits for the operator, in words (proposed/NextStep.md), from
 * the outbox row's `wait_reason`: one vocabulary for the step left, Pending
 * changes, and every outcome that meant to send and didn't.
 *
 *   role_waits                  "{Role} always waits for you"
 *   halted:retrying             "Zitadel didn't answer. Syndra sends it again on its own"
 *                               (the action's `retry` says when: lib/retry.ts)
 *   halted:zitadel_offline      "Zitadel didn't answer at 10:42"
 *   halted:drain_in_progress    "Another send was running at 10:42"
 *   halted:<other>              "The send stopped at 10:42"
 *   manual                      "Its bundle or rule was set to wait" (rows
 *                               from before migration 000078 only: the mode
 *                               it names was dropped there)
 *   refused:<cause>             "Zitadel refused this: {cause}" (not a wait:
 *                               terminal, nothing sends it again; final
 *                               verification N1, lib/retry.ts refusedOutcome)
 */
export function waitReasonLine(
  reason: string | undefined | null,
  role: string,
  at?: string | null,
): string | undefined {
  if (!reason) return undefined;
  if (isRefused(reason)) return `Zitadel refused this: ${refusalCause(reason)}`;
  if (reason === "role_waits") return `${role} always waits for you`;
  if (reason === "manual") return "Its bundle or rule was set to wait";
  const when = at ? ` at ${formatClock(at)}` : "";
  switch (reason) {
    case "halted:retrying":
      return "Zitadel didn't answer. Syndra sends it again on its own";
    case "halted:zitadel_offline":
      return `Zitadel didn't answer${when}`;
    case "halted:drain_in_progress":
      return `Another send was running${when}`;
    default:
      return reason.startsWith("halted:") ? `The send stopped${when}` : undefined;
  }
}

/** A change Zitadel refused outright when it was sent (`refused:<cause>`, db.WaitReasonRefused). */
export function isRefused(reason: string | undefined | null): boolean {
  return Boolean(reason?.startsWith("refused:"));
}

/**
 * The plain cause of a refusal, said after "Zitadel refused this:". The same
 * words as the server's bulk rows (handlers/bulk.go refusalWords).
 */
const REFUSAL_CAUSE: Record<string, string> = {
  not_allowed: "Syndra's service account isn't allowed to make this change",
  not_found: "it no longer has this person, project or role",
  invalid: "it said the change isn't valid",
  other: "it gave a reason Syndra can't put in plain words",
};

export function refusalCause(reason: string): string {
  return REFUSAL_CAUSE[reason.slice("refused:".length)] ?? REFUSAL_CAUSE.other;
}

/** The outbox's wait reason for a refusal by the connected system itself (db.WaitReasonTargetRefused), not Zitadel's `refused:<cause>`. */
export const TARGET_REFUSED = "target_refused";

/**
 * The wait reason of a change the system applied and Syndra could not attribute, because two of
 * Syndra's own records disagree about whose account it landed on (db.WaitReasonBindingConflict).
 * Its own value so no page reads wording to tell it from a refusal.
 */
export const BINDING_CONFLICT = "binding_conflict";

/** A refusal by the connected system itself, or a change it applied that Syndra could not attribute: the two rows that can be a person's account conflict. */
export const isSystemStop = (reason: string | undefined | null) => reason === TARGET_REFUSED || reason === BINDING_CONFLICT;

/**
 * "{system} refused it: {its words} Syndra stopped sending it." The system's
 * words are shown as stored (`last_error`, with no prefix), once.
 */
export function targetRefusedLine(system: string, said: string): string {
  return `${system} refused it: ${sentence(said)} Syndra stopped sending it.`;
}

/**
 * Why Syndra stopped sending a change, in one sentence, for every surface that lists one
 * (Pending changes, Withdrawn access). One path, so none of them prints a stored reason bare, and
 * it switches on the row's `wait_reason`, never on what `last_error` says (the system's name is
 * the page's current one, not the one a stored sentence was written with):
 * - `binding_conflict`: the system applied the change, and Syndra's stored sentence says what is
 *   wrong; never "refused it";
 * - `target_refused`: after "{system} refused it:", the system's words as stored, or none;
 * - Zitadel's `refused:<cause>`: the cause in words;
 * - out of tries: the system did not answer;
 * - any other stop: "{system} refused it", with no reason said that is not its own.
 */
export function stoppedLine(
  system: string,
  row: { last_error?: string; wait_reason?: string; attempts: number },
): string {
  if (row.wait_reason === BINDING_CONFLICT) {
    return `${system} applied the change. ${sentence(row.last_error ?? "") || "Two of Syndra's records disagree about whose account it is. Decide whose it is before changing anything else for them."}`;
  }
  if (row.wait_reason === TARGET_REFUSED) {
    return row.last_error
      ? targetRefusedLine(system, row.last_error)
      : `${system} refused it and gave no reason, so Syndra stopped sending it.`;
  }
  if ((row.last_error ?? "").startsWith("out of retries")) {
    return `${system} didn't answer after ${row.attempts} ${row.attempts === 1 ? "try" : "tries"}, so Syndra stopped sending it.`;
  }
  return `${system} refused it${isRefused(row.wait_reason) ? `: ${refusalCause(row.wait_reason ?? "")}` : ""}, so Syndra stopped sending it.`;
}

/** The next-step seat while the two places a conflict is kept are being read; the generic step is never shown first and then swapped. */
export const LOOKING_UP_NEXT = "Looking up what to do next.";

/** A wait the operator chose (the role, or before the switch its mode), not a send that stopped. */
export function waitIsChosen(reason: string | undefined | null): boolean {
  return reason === "role_waits" || reason === "manual";
}

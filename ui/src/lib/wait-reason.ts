import { formatClock } from "@/lib/format";

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

/** A wait the operator chose (the role, or before the switch its mode), not a send that stopped. */
export function waitIsChosen(reason: string | undefined | null): boolean {
  return reason === "role_waits" || reason === "manual";
}

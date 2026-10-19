import { formatClock } from "@/lib/format";

/**
 * Why a change waits for the operator, in words (proposed/NextStep.md), from
 * the outbox row's `wait_reason`: one vocabulary for the step left, Pending
 * changes, and every outcome that meant to send and didn't.
 *
 *   role_waits                  "{Role} always waits for you"
 *   halted:zitadel_offline      "Zitadel didn't answer at 10:42"
 *   halted:drain_in_progress    "Another send was running at 10:42"
 *   halted:<other>              "The send stopped at 10:42"
 *   manual                      "Its bundle or rule was set to wait"
 */
export function waitReasonLine(
  reason: string | undefined | null,
  role: string,
  at?: string | null,
): string | undefined {
  if (!reason) return undefined;
  if (reason === "role_waits") return `${role} always waits for you`;
  if (reason === "manual") return "Its bundle or rule was set to wait";
  const when = at ? ` at ${formatClock(at)}` : "";
  switch (reason) {
    case "halted:zitadel_offline":
      return `Zitadel didn't answer${when}`;
    case "halted:drain_in_progress":
      return `Another send was running${when}`;
    default:
      return reason.startsWith("halted:") ? `The send stopped${when}` : undefined;
  }
}

/** A wait the operator chose (the role, or before the switch its mode), not a send that stopped. */
export function waitIsChosen(reason: string | undefined | null): boolean {
  return reason === "role_waits" || reason === "manual";
}

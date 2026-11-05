import { formatClock } from "@/lib/format";
import { DRAIN_UNKNOWN_NEXT, NOTHING_TO_DO, SENDS_ITSELF_NEXT, type ActionOutcome, type AutoRetry } from "@/lib/outcome";
import { isRefused, refusalCause } from "@/lib/wait-reason";

/**
 * One retry model, in words (owner, 5 Nov 2026). A Zitadel change in the
 * outbox and a member's storage password are two configurations of the same
 * thing on the server (`internal/extcall` retry.go: OutboxRetry, AddonResend),
 * which answers with this shape wherever a change came back unanswered. Every
 * screen says it through the functions below, so a grant and a password never
 * describe the same situation two ways.
 *
 *   retrying  "Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6)."
 *   waiting   nothing tries again until a person acts: past its last try, or
 *             this server doesn't send changes again on its own
 *             (REVOCATION_DRAIN_ENABLED=false, internal/services/propagation
 *             RowRetry)
 *
 * A change that was sent, or failed, has no retry state: its own status says
 * so (re-audit, 6 Nov 2026: "sent" and "failed" were never produced).
 *
 * Never "Sent" before the system confirms: a retrying change is queued, and
 * goes on its own (`sendsItself`), so it carries no "send it" link.
 */
export type RetryStateName = "retrying" | "waiting";

export interface RetryState {
  state: RetryStateName;
  /** The try that comes next while retrying, otherwise the last one made. */
  try: number;
  of: number;
  /** When it is sent again, while retrying. */
  next_try?: string;
}

/** "at 14:05" in the makerspace's zone, or "in a moment" once that has passed. */
function when(nextTry: string | undefined, now: Date = new Date()): string {
  if (!nextTry) return "shortly";
  const at = new Date(nextTry);
  return at.getTime() <= now.getTime() ? "in a moment" : `at ${formatClock(nextTry)}`;
}

/**
 * The one sentence for a change a system didn't take. `system` is who didn't
 * answer ("Zitadel", "TrueNAS"); `it` is what was sent ("it", "them").
 */
export function retryLine(
  retry: RetryState,
  system = "Zitadel",
  it = "it",
  now: Date = new Date(),
): string {
  switch (retry.state) {
    case "retrying":
      return `${system} didn't answer. Syndra sends ${it} again ${when(retry.next_try, now)} (try ${retry.try} of ${retry.of}).`;
    case "waiting":
      return `${system} didn't answer after ${retry.try === 1 ? "one try" : `${retry.try} tries`}, so Syndra won't send ${it} again on its own.`;
  }
}

/**
 * The step a retry state leaves, or that there is none (owner, 5 Nov 2026):
 * every state ends on one. `own` is a member's own storage password, which
 * only their open page can send again.
 */
export function retryNextStep(retry: RetryState, { own = false, it = "it", now = new Date() }: { own?: boolean; it?: string; now?: Date } = {}): string {
  switch (retry.state) {
    case "retrying":
      return own
        ? "It says here when it's done."
        : `${NOTHING_TO_DO} Syndra sends ${it} again ${when(retry.next_try, now)} and says on Home if it stops sending ${it}.`;
    case "waiting":
      return own ? "Set your password again." : "Press Send on Pending changes to try again.";
  }
}

/** The member's own words for their password while Syndra is still trying (A-13 §4). */
export function memberRetryLine(retry: RetryState, now: Date = new Date()): string {
  return retry.state === "retrying"
    ? `Not confirmed yet. Syndra is trying again ${when(retry.next_try, now)}. Keep this page open.`
    : "Not confirmed yet.";
}

/**
 * An action's outcome when what it sent is retrying: queued, never applied,
 * and on its way by itself. `done` is what happened in Syndra ("Approved").
 */
export function retryingOutcome(done: string, retry: RetryState | undefined, it = "it"): ActionOutcome {
  if (!retry) {
    // The server couldn't read the row's next try (services/delivery.go
    // nextRetry), so neither "on its own" nor "waits for Send" is known (N3).
    const line = `Zitadel didn't answer, so ${it} isn't in Zitadel yet.`;
    return { kind: "queued", message: `${done}. ${line}`, detail: line, sendsItself: true, next: DRAIN_UNKNOWN_NEXT };
  }
  const line = retryLine(retry, "Zitadel", it);
  // Waiting is never "on its way": nothing sends it until Send is pressed.
  const goes = retry.state === "retrying";
  return {
    kind: "queued",
    message: `${done}. ${line}`,
    detail: line,
    sendsItself: goes || undefined,
    next: retryNextStep(retry, { it }),
  };
}

/** The wait reason an action answers with when what it sent is retrying. */
export const RETRYING = "halted:retrying";

/** Where Syndra lists a change it stopped sending (pending/page.tsx StoppedSending). */
const STOPPED = "It isn't in Zitadel, and Syndra won't send it again. Pending changes lists it under Syndra stopped sending.";

/**
 * The one true step a refusal leaves (final verification N1): the change is
 * terminal, so Send won't move it. A give is given again from the person's
 * page; a revocation still held shows in Drift after the next check and is
 * revoked there (the same steps as Pending changes' Syndra stopped sending).
 * `again` names the act to repeat ("give", "extend").
 */
export function refusedNext(reason: string | undefined, { revocation = false, again = "give" }: { revocation?: boolean; again?: string } = {}): string {
  const step = revocation
    ? "They may still hold it in Zitadel. It shows in Drift after Syndra's next check; revoke it there once the reason is fixed."
    : `Open their page to see what they hold now, then ${again} it again once the reason is fixed.`;
  return reason === "refused:not_allowed" ? `Ask whoever runs Zitadel to let Syndra's service account change grants. ${step}` : step;
}

/**
 * An action whose change Zitadel refused outright (`refused:<cause>`): done in
 * Syndra, not in Zitadel, and never sent again. Says "Zitadel refused this",
 * the plain cause, and the true step. `done` is what happened in Syndra.
 */
export function refusedOutcome(
  done: string,
  reason: string | undefined,
  options: { revocation?: boolean; again?: string } = {},
): ActionOutcome {
  return {
    kind: "failed",
    recorded: true,
    message: `${done}. Zitadel refused this: ${refusalCause(reason ?? "refused:other")}`,
    detail: STOPPED,
    next: refusedNext(reason, options),
  };
}

/**
 * What retiring a bundle or a rule did to the access it gave: `n` revocations,
 * split by why any are not in Zitadel yet (W1, W3). A role that waits holds
 * them for the operator; a send that stopped, or that Zitadel didn't take,
 * goes on its own, because revocations do. Shared, so a bundle and a rule say
 * it the same way. `after` is appended to the detail.
 */
export function retireOutcome(
  n: number,
  reason: string | undefined,
  retry?: RetryState,
  after = "",
  auto: AutoRetry = true,
): ActionOutcome {
  const changes = `${n} ${n === 1 ? "change" : "changes"}`;
  if (!reason) {
    return { kind: "applied", message: `${changes} went to Zitadel.`, detail: `The access it gave was revoked.${after}` };
  }
  // Final verification N1: Zitadel refused a revocation outright. Terminal:
  // never "went", never "on its way".
  if (isRefused(reason)) {
    return {
      kind: "failed",
      recorded: true,
      message: `The access it gave was revoked in Syndra. Zitadel refused this: ${refusalCause(reason)}`,
      detail: `What Zitadel refused isn't in Zitadel, and Syndra won't send it again. Pending changes lists it under Syndra stopped sending.${after}`,
      next: refusedNext(reason, { revocation: true }),
    };
  }
  // Before the indicators are read nothing is promised either way (N3),
  // unless the server already said so with its own retry state (below).
  if (auto === null && reason !== "role_waits" && reason !== "manual" && !retry) {
    return {
      kind: "queued",
      message: `Not in Zitadel yet. Until ${n === 1 ? "it is" : "they are"}, that access stays.`,
      detail: after.trim() || undefined,
      sendsItself: true,
      next: DRAIN_UNKNOWN_NEXT,
    };
  }
  // With the background drain off nothing goes on its own, so a revocation
  // that is not in Zitadel waits for Send like one a role holds, unless the
  // server already said so with its own retry state (below).
  if (reason === "role_waits" || reason === "manual" || (auto === false && retry?.state !== "waiting")) {
    return {
      kind: "queued",
      message: `${n} ${n === 1 ? "change waits" : "changes wait"} for you to send.`,
      detail: `The revocations wait under Pending changes until you send them. Until then, that access stays.${after}`,
    };
  }
  const line =
    reason === RETRYING && retry
      ? retryLine(retry, "Zitadel", "them")
      : "Zitadel didn't answer, so Syndra sends them again on its own.";
  if (retry?.state === "waiting") {
    // This server sends nothing again on its own (the background drain is off).
    return { kind: "queued", message: `Not in Zitadel yet. ${line}`, detail: after.trim() || undefined, next: retryNextStep(retry, { it: "them" }) };
  }
  return {
    kind: "queued",
    message: `Not in Zitadel yet. ${line}`,
    detail: `Revocations send on their own; nothing waits for you.${after}`,
    sendsItself: true,
    next: reason === RETRYING && retry ? retryNextStep(retry, { it: "them" }) : SENDS_ITSELF_NEXT,
  };
}

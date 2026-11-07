import { ApiError, LostReply } from "@/lib/api-client";

/**
 * What happened, in six words, everywhere in the product.
 *
 * A mutation's result used to be a corner toast that appeared for four seconds
 * and then took the only account of what happened with it. It is now reported
 * by the surface that ran the action — the row reports its own row, a sheet
 * becomes its own result, a plan reports on its result step — and this module
 * is the vocabulary all three share so no screen invents a seventh word.
 *
 * The two halves are deliberately different sets. A PLAN describes what would
 * happen and can say `apply`; a RESULT describes what did happen and cannot.
 * Collapsing them is how "will change" ends up reported as though it had.
 */

/** The effects a plan row can carry. `apply` is the future tense. */
export type PlanEffect = "apply" | "applied" | "no_change" | "blocked" | "failed" | "queued";

/** What an action can turn out to be. No `apply`: this is the past tense. */
export type OutcomeKind = "applied" | "queued" | "no_change" | "refused" | "failed";

export interface ActionOutcome {
  kind: OutcomeKind;
  /**
   * What happened, in physical terms. "She loses badge entry to the laser bay",
   * never "grant deleted" — the operator is accountable for the consequence,
   * not for the row that recorded it.
   */
  message: string;
  /**
   * The nuance behind the headline, when there is any — which rows were left
   * queued, what resuming will and will not pick up. A drain has this almost
   * always and a single grant almost never.
   */
  detail?: string;
  /** Present on refused and failed, so it can be quoted to whoever runs this. */
  requestId?: string;
  /** The session ended before this ran: offer to sign in again in a new tab (N21). */
  sessionEnded?: boolean;
  /**
   * Queued, but nobody has to send it: a revocation drains on its own (the
   * background revocation drain, design §7). Said as "On its way", with a
   * neutral dot and no link, because there is no step left for the reader.
   */
  sendsItself?: boolean;
  /**
   * The write may have partly landed, so "Nothing was changed." would be false.
   * The place to check (MAY_HAVE_GONE_THROUGH), said by {@link partialSentence}.
   */
  partial?: string;
  /**
   * The step this outcome leaves, or that there is none, when the caller knows
   * better than the default for its kind ({@link nextStepOf}). Every outcome
   * ends on one (owner, 5 Nov 2026: "clear next steps or lack thereof").
   */
  next?: string;
  /**
   * Syndra recorded the act, and only the target refused it (final
   * verification N1, `refused:<cause>`). A failure, but never "Nothing was
   * changed.", and the act's own button is spent: approving or giving again
   * is not the step (lib/retry.ts refusedOutcome says the step).
   */
  recorded?: boolean;
}

/** The explicit "no step left" (copy guide §2): said, never implied by silence. */
export const NOTHING_TO_DO = "Nothing for you to do.";

/** The step a change waiting for its send leaves; ActionOutcome draws it as the NextStep link. */
export const SEND_IT = "Press Send on Pending changes.";

/** A change that goes on its own, with nothing to say about when. */
export const SENDS_ITSELF_NEXT = `${NOTHING_TO_DO} Syndra sends it on its own and says on Home if it stops sending it.`;

/**
 * Whether this server sends changes again on its own (lib/queries/useIndicators
 * useAutoRetry): null until the indicators are read, or when they can't be
 * (final verification N3). Unknown promises nothing either way.
 */
export type AutoRetry = boolean | null;

/**
 * The step when Syndra can't tell yet whether a change goes on its own or
 * waits for Send (N3): true either way, because Pending changes lists
 * everything that has not reached Zitadel.
 */
export const DRAIN_UNKNOWN_NEXT = "Open Pending changes to see what has not reached Zitadel yet.";

/**
 * The step an outcome leaves, or that there is none. Never empty: the guard
 * test (`next-step.test.ts`) walks every kind. The caller's own `next` wins;
 * otherwise the kind decides.
 */
export function nextStepOf(outcome: ActionOutcome): string {
  if (outcome.next) return outcome.next;
  if (outcome.partial) return partialSentence(outcome.partial);
  switch (outcome.kind) {
    case "applied":
    case "no_change":
      // Only true when nothing is left: a caller whose detail names a step,
      // or whose change left anything waiting, says so in `next` (re-audit
      // R1, R4, R6, R8, R12); reaudit-outcomes.test.tsx holds each.
      return NOTHING_TO_DO;
    case "queued":
      return outcome.sendsItself ? SENDS_ITSELF_NEXT : SEND_IT;
    case "refused":
      return outcome.sessionEnded
        ? "Sign in again, then press the same button once more."
        : "Change what it says above, then try again.";
    case "failed":
      return nextStep(Boolean(outcome.requestId));
  }
}

/**
 * The one place a tone is attached to an outcome.
 *
 * `queued` takes the warn tone rather than the accent one, and that is the most
 * important line in this file: queued is not succeeded. Syndra recorded the
 * decision and the target has not been told. An accent pill and a past-tense
 * verb would report a door as locked while it still opens.
 */
export const OUTCOME_TONE: Record<OutcomeKind, string> = {
  applied: "bg-accent-soft text-accent-text",
  queued: "bg-warn-soft text-warn-text",
  no_change: "bg-tint-2 text-muted",
  refused: "bg-warn-soft text-warn-text",
  failed: "bg-danger-soft text-danger-text",
};

export const OUTCOME_LABEL: Record<OutcomeKind, string> = {
  applied: "Done",
  queued: "Waiting for you to send",
  no_change: "No change",
  refused: "Refused",
  failed: "Failed",
};

/** The same six, as a plan renders them. `apply` reads as the future it is. */
export const PLAN_EFFECT_LABEL: Record<PlanEffect, string> = {
  apply: "Will change",
  applied: "Done",
  no_change: "No change",
  blocked: "Refused",
  failed: "Failed",
  // The future tense, like "Will change": nothing has been queued yet, so a
  // plan row can't wear the link a queued result does (A-06 §4). It becomes
  // "Waiting for you to send →" in the result, once it is true.
  queued: "Will wait for you to send",
};

/** A plan row's Badge: quiet for what will simply happen, a tint for what stops or waits. */
export const PLAN_EFFECT_TONE = {
  apply: "neutral",
  applied: "neutral",
  no_change: "neutral",
  blocked: "warnSoft",
  failed: "dangerSoft",
  queued: "warnSoft",
} as const satisfies Record<PlanEffect, string>;

/** Ends a fragment with a full stop so it does not run into the next sentence. */
export function sentence(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return "";
  return /[.!?]$/.test(trimmed) ? trimmed : `${trimmed}.`;
}

/** What a browser calls a request that never reached the server, in each engine's words. */
const RAW_NETWORK = /failed to fetch|networkerror|network request failed|load failed|fetch failed/i;

export const NETWORK_FAULT = "Syndra's server didn't answer.";

/** The third of the copy guide's four parts (§8): the next step, and where. */
export function nextStep(hasReference: boolean): string {
  return `Try again in a minute. If it keeps failing, ask the person who runs the Syndra server${hasReference ? " and quote the reference below" : ""}.`;
}

/** What went wrong, in the product's voice rather than the transport's. */
export function failureMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 403) return "You don't have permission to do this.";
    if (error.status === 404) return "That item no longer exists. Someone may have removed it.";
    return error.message;
  }
  // A fetch that never connected: the browser's words are never the product's.
  if (error instanceof TypeError || (error instanceof Error && RAW_NETWORK.test(error.message))) return NETWORK_FAULT;
  if (error instanceof Error) return error.message;
  return "Something went wrong and the request did not go through.";
}

/**
 * {@link failureMessage} for a line that stands alone. A server fault says what
 * to do and carries its reference in full, because a bare line has no card
 * beneath it to hold them (copy guide §8). Surfaces that show the reference
 * themselves use failureMessage.
 */
export function describeFailure(error: unknown): string {
  const said = failureMessage(error);
  const where = mayHaveGoneThrough(error);
  if (!where && (!(error instanceof ApiError) || error.status < 500)) return said;
  const ref = error instanceof ApiError ? error.details?.request_id : undefined;
  // A write that may have landed is not told to simply try again: check first.
  const step = where
    ? `${partialSentence(where)} If it keeps failing, ask the person who runs the Syndra server`
    : "Try again in a minute. If it keeps failing, ask the person who runs the Syndra server";
  return `${sentence(said)} ${step}${typeof ref === "string" ? ` and quote the reference ${ref}` : ""}.`;
}

/**
 * A thrown error, as an outcome.
 *
 * Refused and failed are kept apart because they send an operator to different
 * places. A refusal is Syndra declining — the plan went stale, the cohort was
 * too large, a confirmation was missing — and the operator can act on it. A
 * failure is the machinery not answering, and the operator can only quote the
 * id. A 4xx carries a reason somebody wrote; anything else does not.
 */
export function outcomeFromError(error: unknown): ActionOutcome {
  const requestId = error instanceof ApiError ? error.details?.request_id : undefined;
  // Offline is a refusal, not a failure. Nothing was attempted, so the state
  // on screen is the state that holds — which is precisely what an operator
  // needs to know, and the opposite of what a failure tells them.
  const offline = error instanceof ApiError && error.code === "OFFLINE";
  const sessionEnded = error instanceof ApiError && error.code === "SESSION_ENDED";
  const refused =
    offline ||
    sessionEnded ||
    (error instanceof ApiError && error.status >= 400 && error.status < 500);
  const where = mayHaveGoneThrough(error);
  return {
    kind: refused ? "refused" : "failed",
    partial: where,
    message: failureMessage(error),
    // Not "it will send when you're back". There is no client-side queue by
    // design, so nothing sends itself, and saying otherwise would be the
    // product promising something it has deliberately refused to build.
    detail: offline
      ? "Nothing was sent. Try again once you are back online."
      : sessionEnded
        // "Nothing was changed" is not repeated here: every refusal already
        // ends with it. "Still here" is what stops an operator abandoning a
        // plan they had already read and approved.
        ? "What you were doing is still here. Sign in again, then press the same button once more."
        : // A server fault ends on the next step and where (copy guide §8); the reference sits below.
          where
          ? `${partialSentence(where)} ${nextStep(typeof requestId === "string")}`
          : error instanceof ApiError && error.status >= 500
            ? nextStep(typeof requestId === "string")
            : undefined,
    requestId: typeof requestId === "string" ? requestId : undefined,
    sessionEnded: sessionEnded || undefined,
    // The step, by what refused it; the rest fall to nextStepOf's kind default.
    next: offline
      ? "Try again once you are back online."
      : error instanceof ApiError && error.status === 403
        ? "Ask someone who runs Syndra to do it, or to give you access."
        : error instanceof ApiError && error.status === 404
          ? "Reload the page to see what is there now."
          : undefined,
  };
}

/** How long an extension buys. Stated in the row's own result, never implied. */
export const EXTEND_DAYS = 90;

/**
 * The result of extending a grant, as an outcome. Extending is a ledger edit
 * (architecture deepening 1.2a): Zitadel has no end dates and Syndra ends the
 * access on the day, so nothing is sent and nothing waits; it is done when the
 * write answers. `said` is the plan's own sentence ("Keeps Member in Laser Lab
 * until 3 Dec 2026."); `leaves` is the closing line for a surface whose row
 * goes on the next read.
 */
export function outcomeFromExtend(days: number, said?: string, leaves?: string): ActionOutcome {
  const detail = [said, leaves].filter(Boolean).join(" ");
  return { kind: "applied", message: `Extended by ${days} days`, ...(detail ? { detail } : {}) };
}

/**
 * The first 100ms, in the plan's own tense (A-19 P2). While a write is in
 * flight the seat says what Syndra WILL do, never that it did: a forecast is a
 * sentence about Syndra's own next step, which Syndra knows; whether the target
 * has the change is observed later and said in the true word. Words, not
 * counts: no number on screen moves until the server has answered.
 */
export const FORECAST = {
  /** An act Syndra already knows will wait for the operator. Same words as a plan row. */
  queued: PLAN_EFFECT_LABEL.queued,
  approve: "Will be approved",
  decline: "Will be declined",
  extend: "Will be extended",
  remove: "Will be removed",
  keep: "Will be kept",
  lift: "Will be lifted",
  give: "Will be given",
  cancel: "Will be cancelled",
  adopt: "Will be adopted",
  mark: "Will be marked",
  /** The action signing key: Zitadel makes a new one and Syndra uses it at once. */
  replace: "Will be replaced",
} as const;

/** The acts a forecast can name: every word but the one for a wait. */
export type ForecastAct = Exclude<keyof typeof FORECAST, "queued">;

/**
 * Acts that go on to Zitadel. When the role they touch is one of Roles that
 * wait, Syndra already knows they will wait for the operator, so the forecast
 * is the wait and not the act (A-20 P8): "Will wait for you to send", never
 * "Will be approved". Decline, keep, lift, cancel and mark never reach Zitadel.
 * Adopt does reach it, indirectly: it sends nothing of its own row, but it
 * fires the automatic rules, and each role a rule gives is queued in Pending
 * changes for you to send (drift attribute answers `rule_outbox_ids`). Its own
 * forecast is still "Will be adopted", so a waiting role changes nothing here.
 */
const REACHES_ZITADEL: ReadonlySet<ForecastAct> = new Set<ForecastAct>(["approve", "extend", "remove", "give"]);

/** The forecast seat's word for `act`, given whether its role is a role that waits. */
export function forecastFor(act: ForecastAct, roleWaits?: boolean): string {
  return roleWaits && REACHES_ZITADEL.has(act) ? FORECAST.queued : FORECAST[act];
}

/**
 * A Drift revoke, as an outcome. Code decides, and code says it sends by
 * itself: the revoke is enqueued as op_type "revoke", which the background
 * drain claims within minutes with no operator in the loop (internal/db
 * ClaimPendingRevocations). So this never wears the amber "Waiting for you to
 * send" link: there is nothing for the reader to do.
 */
export const REVOKE_SENDS_ITSELF: ActionOutcome = {
  kind: "queued",
  message: "Revocations send on their own",
  sendsItself: true,
};
export const REVOKE_SENDS_ITSELF_WORD = "On its way";

/**
 * The word for a change that carries no Send link: "On its way" only when it
 * really goes on its own; "Not in Zitadel yet" while that isn't known (N3).
 */
export function sendsItselfWord(outcome: ActionOutcome): string {
  return outcome.next === DRAIN_UNKNOWN_NEXT ? "Not in Zitadel yet" : REVOKE_SENDS_ITSELF_WORD;
}

/**
 * The same revoke when this server's background drain is off
 * (REVOCATION_DRAIN_ENABLED=false, `auto_retry: false` in the indicators):
 * nothing sends it until someone presses Send, so it says that and ends on
 * Pending changes.
 */
export const REVOKE_WAITS_FOR_SEND: ActionOutcome = {
  kind: "queued",
  message: "Recorded in Syndra. The revocation waits in Pending changes until someone presses Send",
};

/** The same revoke before Syndra knows whether the drain is on (N3): no promise either way. */
export const REVOKE_NOT_YET: ActionOutcome = {
  kind: "queued",
  message: "Recorded in Syndra. The revocation is not in Zitadel yet",
  sendsItself: true,
  next: DRAIN_UNKNOWN_NEXT,
};

/** {@link REVOKE_SENDS_ITSELF} with the drain on, {@link REVOKE_WAITS_FOR_SEND} with it off, {@link REVOKE_NOT_YET} unknown. */
export const revokeQueued = (auto: AutoRetry): ActionOutcome =>
  auto === null ? REVOKE_NOT_YET : auto ? REVOKE_SENDS_ITSELF : REVOKE_WAITS_FOR_SEND;

/**
 * Every refusal and every failure ends the same way, and it is the sentence an
 * operator most needs: whatever else went wrong, the state they were looking at
 * is the state that still holds.
 */
export const NOTHING_CHANGED = "Nothing was changed.";

/**
 * Failures of a write that does several things, or whose answer may be lost
 * after the thing was done. Each is told with where to look (copy guide §8:
 * say the state of the world, then the next step) and never with
 * {@link NOTHING_CHANGED}. Every backend code that can fail part-way is here;
 * `partial-codes.test.ts` reads the handlers and fails on one that is not.
 */
/**
 * Where to look after a write whose reply never came. Any action can lose its
 * reply, and not every one leaves a row in Pending changes or Audit (marking a
 * give-up seen, a setting), so the place is the one every write changes: the
 * page it was made on, read again.
 */
const LOST_REPLY_WHERE = "this page after a reload";

export const MAY_HAVE_GONE_THROUGH: Record<string, string> = {
  DRAIN_ERROR: "Pending changes",
  CASCADE_ERROR: "Pending changes",
  APPLY_ERROR: "Pending changes and the bundle's page",
  RESOLVE_FAILED: "Pending changes and Audit",
  CONVERGENCE_NOT_QUEUED: "Pending changes",
  CONVERGENCE_CHECK_FAILED: "Pending changes",
  RECONCILE_FAILED: "the system's page under System",
  LIFECYCLE_NOT_SET: "the system's page under System",
  RELEASE_NOT_CONFIRMED: "the system's page under System",
  TARGET_UNCONFIRMED: "the system's page under System",
  TARGET_ANSWER_UNREADABLE: "the system's page under System",
  SWEEP_ERROR: "Audit",
  // A refused role write whose undo failed: Zitadel did not change, but Syndra's own list of roles may still show it (services/roles.go writeRole).
  RECORD_NOT_RESTORED: "the role under Roles",
  // Writes the answer and its audit line in one transaction (db/addon_operations.go:382-419); a failed commit is unknowable from here.
  ANSWER_NOT_RECORDED: "Unconfirmed changes on the system's page",
  // A role write whose answer was lost and whose read-back failed (handlers/discovery.go roleChangeUnknownCode): it may be in Zitadel.
  OUTCOME_UNKNOWN: "Zitadel › Projects",
  // A Zitadel › Users change that recorded some roles, then hit a fault (handlers/discovery.go grantChangeFailed).
  PARTLY_RECORDED: "Pending changes and the person's page",
  // Read on most screens, write on a few (WRITE_ONLY).
  ZITADEL_ERROR: "Audit",
  INTERNAL_ERROR: "Audit",
  // The proxy gave up, or a gateway answered for the backend: the write may have landed.
  BACKEND_UNREACHABLE: LOST_REPLY_WHERE,
  UPSTREAM_ERROR: LOST_REPLY_WHERE,
};

/** Codes that stand for a read on one screen and a write on another: partial only for a write. */
const WRITE_ONLY = new Set(["ZITADEL_ERROR", "INTERNAL_ERROR", "BACKEND_UNREACHABLE", "UPSTREAM_ERROR"]);

/** Where to check, when this error may have partly landed; otherwise undefined. */
export function mayHaveGoneThrough(error: unknown): string | undefined {
  if (error instanceof LostReply) return LOST_REPLY_WHERE;
  if (!(error instanceof ApiError) || error.status < 500) return undefined;
  const where = MAY_HAVE_GONE_THROUGH[error.code];
  if (!where) return undefined;
  return WRITE_ONLY.has(error.code) && (error.method ?? "GET") === "GET" ? undefined : where;
}

/** What a failed write says about the state of the world: the partial sentence, or "Nothing was changed." */
export function stateLine(error: unknown): string {
  const where = mayHaveGoneThrough(error);
  return where ? partialSentence(where) : NOTHING_CHANGED;
}

/** The sentence that replaces "Nothing was changed." for a partial failure. */
export function partialSentence(where: string): string {
  return `Some of this may have gone through. Check ${where} before trying again.`;
}

/** True when the outcome must carry {@link NOTHING_CHANGED}. */
export function statesNothingChanged(kind: OutcomeKind): boolean {
  return kind === "refused" || kind === "failed";
}

/**
 * True when the step left is pressing Send, drawn as the one link "Waiting for
 * you to send →". A queued outcome that goes on its own, or whose caller named
 * another step (a refused change that Send won't move), draws its own line.
 */
export function waitsForSend(outcome: ActionOutcome): boolean {
  return outcome.kind === "queued" && !outcome.sendsItself && (outcome.next === undefined || outcome.next === SEND_IT);
}

/**
 * Whether the action ran. A FAILED attempt keeps its button — retiring the
 * control on any outcome at all would leave a refusal on screen with no way to
 * try again.
 */
export function succeeded(outcome: ActionOutcome | null): boolean {
  return outcome !== null && (!statesNothingChanged(outcome.kind) || Boolean(outcome.recorded));
}

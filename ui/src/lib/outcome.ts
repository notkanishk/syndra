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
}

/** The explicit "no step left" (copy guide §2): said, never implied by silence. */
export const NOTHING_TO_DO = "Nothing for you to do.";

/** The step a change waiting for its send leaves; ActionOutcome draws it as the NextStep link. */
export const SEND_IT = "Press Send on Pending changes.";

/** A change that goes on its own, with nothing to say about when. */
export const SENDS_ITSELF_NEXT = `${NOTHING_TO_DO} Syndra sends it on its own and says on Home if it gives up.`;

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
 * The result of extending a grant, as an outcome. The grant endpoint enqueues to
 * the outbox in manual mode ("pending") rather than applying inline, so reporting
 * "extended" unconditionally would say Zitadel has it when only Syndra's own
 * ledger does. `leaves` is the closing line for a surface whose row goes on the
 * next read; it is not said while the change is still held back.
 */
export function outcomeFromExtend(
  result: { status?: string; outbox_id?: string } | undefined,
  days: number,
  leaves?: string,
): ActionOutcome {
  const waiting = result?.status === "pending";
  return {
    kind: waiting ? "queued" : "applied",
    message: waiting ? `Will extend by ${days} days once sent` : `Extended by ${days} days`,
    detail: waiting ? undefined : leaves,
  };
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
/** Where to look after a write whose reply never came: any action, so the two places every one of them leaves a trace. */
const LOST_REPLY_WHERE = "Pending changes or Audit";

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
  // Writes the answer and its audit line in one transaction (db/addon_operations.go:382-419); a failed commit is unknowable from here.
  ANSWER_NOT_RECORDED: "Unconfirmed changes on the system's page",
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
 * Whether the action ran. A FAILED attempt keeps its button — retiring the
 * control on any outcome at all would leave a refusal on screen with no way to
 * try again.
 */
export function succeeded(outcome: ActionOutcome | null): boolean {
  return outcome !== null && !statesNothingChanged(outcome.kind);
}

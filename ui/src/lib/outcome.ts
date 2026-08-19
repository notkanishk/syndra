import { ApiError } from "@/lib/api-client";

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
  applied: "Applied",
  queued: "Queued",
  no_change: "No change",
  refused: "Refused",
  failed: "Failed",
};

/** The same six, as a plan renders them. `apply` reads as the future it is. */
export const PLAN_EFFECT_LABEL: Record<PlanEffect, string> = {
  apply: "Will change",
  applied: "Applied",
  no_change: "No change",
  blocked: "Refused",
  failed: "Failed",
  queued: "Queued",
};

export const PLAN_EFFECT_TONE: Record<PlanEffect, string> = {
  apply: OUTCOME_TONE.applied,
  applied: OUTCOME_TONE.applied,
  no_change: OUTCOME_TONE.no_change,
  blocked: OUTCOME_TONE.refused,
  failed: OUTCOME_TONE.failed,
  queued: OUTCOME_TONE.queued,
};

/** Ends a fragment with a full stop so it does not run into the next sentence. */
export function sentence(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return "";
  return /[.!?]$/.test(trimmed) ? trimmed : `${trimmed}.`;
}

/** What went wrong, in the product's voice rather than the transport's. */
export function describeFailure(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 403) return "You don't have access to this.";
    if (error.status === 404) return "It no longer exists.";
    return error.message;
  }
  if (error instanceof Error) return error.message;
  return "The request failed.";
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
  const refused = error instanceof ApiError && error.status >= 400 && error.status < 500;
  return {
    kind: refused ? "refused" : "failed",
    message: describeFailure(error),
    requestId: typeof requestId === "string" ? requestId : undefined,
  };
}

/**
 * Every refusal and every failure ends the same way, and it is the sentence an
 * operator most needs: whatever else went wrong, the state they were looking at
 * is the state that still holds.
 */
export const NOTHING_CHANGED = "Nothing was changed.";

/** True when the outcome must carry {@link NOTHING_CHANGED}. */
export function statesNothingChanged(kind: OutcomeKind): boolean {
  return kind === "refused" || kind === "failed";
}

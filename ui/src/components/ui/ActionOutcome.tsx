"use client";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { ARROW_LINK } from "@/components/ui/ArrowLink";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { NextStep } from "@/components/ui/NextStep";
import {
  NOTHING_CHANGED,
  OUTCOME_LABEL,
  OUTCOME_TONE,
  sentence,
  statesNothingChanged,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";

/**
 * What happened, reported by the surface that made it happen.
 *
 * The rule this component exists to keep: a result never travels. A row
 * reports in the row, a sheet becomes its own result, a plan reports on its
 * result step. A corner toast broke that rule by definition — it reported
 * every action in the same place regardless of where the operator was looking,
 * covered the value they had just acted on, and then removed the only account
 * of what happened after four seconds.
 *
 * Two placements, one vocabulary:
 *
 *  - `inline` sits in a row's own corner. The row keeps its seat and dims; it
 *    leaves on the next read, never on the tap, because a row that vanishes
 *    under a thumb takes its own evidence with it.
 *  - `block` is a sheet or panel becoming its result, with room for the
 *    request id and the sentence that follows a refusal.
 *  - `seat` takes the slot a row's buttons held (proposed/ActionOutcome-seat.md):
 *    a dot and the decision in the row's own word, then where it went.
 *
 * Queued is never a pill. It is the step left, said where it was taken: the
 * one link "Waiting for you to send →", with the message saying what happened
 * ("Given in Syndra") and the detail saying why it waits.
 */
const SEAT_DOT: Record<Outcome["kind"], StatusTone> = {
  applied: "healthy",
  queued: "warn",
  no_change: "neutral",
  refused: "danger",
  failed: "danger",
};

export function ActionOutcome({
  outcome,
  placement = "block",
  word,
  className = "",
}: {
  outcome: Outcome;
  placement?: "inline" | "block" | "seat";
  /** The decision in the row's own vocabulary ("Approved", "You withdrew this"). Seat only. */
  word?: string;
  className?: string;
}) {
  const grave = outcome.kind === "failed" || outcome.kind === "refused";

  if (placement === "seat") {
    return (
      <span
        role={grave ? "alert" : "status"}
        className={`tint-in flex flex-col gap-1 ${className}`}
      >
        <span className="inline-flex items-center gap-2 text-[13.5px] font-semibold text-ink">
          <StatusDot tone={SEAT_DOT[outcome.kind]} />
          {word ?? OUTCOME_LABEL[outcome.kind]}
        </span>
        {/* Queued: why it waits, then the step left — the word above already
            wears the amber dot, so the link doesn't repeat it (route-requests). */}
        {(grave ? outcome.message : outcome.detail) && (
          <span className="pl-[14px] text-[12.5px] leading-[1.45] text-faint">
            {grave ? sentence(outcome.message) : outcome.detail}
          </span>
        )}
        {outcome.kind === "queued" && <NextStep dot={false} className="pl-[14px]" />}
        {outcome.kind === "failed" && outcome.requestId && (
          <CopyableValue value={outcome.requestId} label="Reference" className="pl-[14px]" />
        )}
      </span>
    );
  }

  if (placement === "inline") {
    return (
      <span
        // `status`, not `alert`: an outcome the operator caused by tapping is
        // not an interruption, and an alert would talk over what they are
        // already reading. Failures get `alert` in the block form below, where
        // the operator may have looked away.
        role="status"
        className={`tint-in inline-flex flex-wrap items-center gap-x-2.5 gap-y-1 ${className}`}
      >
        <OutcomeMark kind={outcome.kind} />
        <span className="text-[13px] text-muted">{sentence(outcome.message)}</span>
      </span>
    );
  }

  return (
    <div
      role={grave ? "alert" : "status"}
      className={`tint-in flex flex-col gap-2.5 rounded-inner border px-5 py-4 ${
        grave ? "border-danger-line bg-surface-1" : "border-line-strong bg-surface-1"
      } ${className}`}
    >
      <div className="flex flex-wrap items-center gap-2.5">
        <OutcomeMark kind={outcome.kind} />
      </div>

      <p className="text-[14px] leading-[1.55] text-muted">
        {sentence(outcome.message)}
        {statesNothingChanged(outcome.kind) ? ` ${NOTHING_CHANGED}` : ""}
      </p>

      {outcome.detail && (
        <p className="text-[13px] leading-[1.5] text-faint">{outcome.detail}</p>
      )}

      {/* N21, 21b: sign in again without leaving this tab, so the plan, the
          reason typed and the dialog itself are all still here after. */}
      {outcome.sessionEnded && (
        <div className="flex flex-col gap-1">
          <a
            href="/auth/zitadel?then=close"
            target="_blank"
            rel="noopener"
            className={`${ARROW_LINK} self-start`}
          >
            Sign in again in a new tab <span aria-hidden>→</span>
          </a>
          <p className="text-[12.5px] text-faint">
            This tab stays as it is. Come back here when you&rsquo;re signed in.
          </p>
        </div>
      )}

      {/* Labelled, because an operator pasting this into a message needs to
          say what it is, and a bare hex string in a chat window is a riddle
          for whoever receives it. */}
      {outcome.requestId && (
        <div className="flex flex-col gap-1.5">
          <span className="type-label">Quote this if you ask for help</span>
          <CopyableValue value={outcome.requestId} label="Reference" />
        </div>
      )}
    </div>
  );
}

/** The pill, or for queued the step left in its place. */
function OutcomeMark({ kind }: { kind: Outcome["kind"] }) {
  if (kind === "queued") return <NextStep />;
  return (
    <span className={`rounded-pill px-2.5 py-1 text-[12.5px] font-semibold ${OUTCOME_TONE[kind]}`}>
      {OUTCOME_LABEL[kind]}
    </span>
  );
}

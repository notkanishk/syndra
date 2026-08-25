"use client";

import { CopyableValue } from "@/components/ui/CopyableValue";
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
 */
export function ActionOutcome({
  outcome,
  placement = "block",
  className = "",
}: {
  outcome: Outcome;
  placement?: "inline" | "block";
  className?: string;
}) {
  if (placement === "inline") {
    return (
      <span
        // `status`, not `alert`: an outcome the operator caused by tapping is
        // not an interruption, and an alert would talk over what they are
        // already reading. Failures get `alert` in the block form below, where
        // the operator may have looked away.
        role="status"
        className={`inline-flex items-center gap-2 ${className}`}
      >
        <span
          className={`rounded-pill px-2.5 py-1 text-[12.5px] font-semibold ${OUTCOME_TONE[outcome.kind]}`}
        >
          {OUTCOME_LABEL[outcome.kind]}
        </span>
        <span className="text-[13px] text-muted">{sentence(outcome.message)}</span>
      </span>
    );
  }

  const grave = outcome.kind === "failed" || outcome.kind === "refused";

  return (
    <div
      role={grave ? "alert" : "status"}
      className={`flex flex-col gap-2.5 rounded-inner border px-5 py-4 ${
        grave ? "border-danger-line bg-surface-1" : "border-line-strong bg-surface-1"
      } ${className}`}
    >
      <div className="flex flex-wrap items-center gap-2.5">
        <span
          className={`rounded-pill px-2.5 py-1 text-[12.5px] font-semibold ${OUTCOME_TONE[outcome.kind]}`}
        >
          {OUTCOME_LABEL[outcome.kind]}
        </span>
      </div>

      <p className="text-[14px] leading-[1.55] text-muted">
        {sentence(outcome.message)}
        {statesNothingChanged(outcome.kind) ? ` ${NOTHING_CHANGED}` : ""}
      </p>

      {outcome.detail && (
        <p className="text-[13px] leading-[1.5] text-faint">{outcome.detail}</p>
      )}

      {/* Labelled, because an operator pasting this into a message needs to
          say what it is, and a bare hex string in a chat window is a riddle
          for whoever receives it. */}
      {outcome.requestId && (
        <div className="flex flex-col gap-1.5">
          <span className="type-label">Quote this if you ask for help</span>
          <CopyableValue value={outcome.requestId} label="Request id" />
        </div>
      )}
    </div>
  );
}

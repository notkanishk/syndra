import { targetLabel } from "@/lib/nav";
import type { ActionOutcome, OutcomeKind } from "@/lib/outcome";
import type { DrainResult } from "@/lib/queries/usePropagation";

export type DrainTone = "success" | "warning" | "error" | "info";

export interface DrainOutcome {
  tone: DrainTone;
  /** One scannable line. */
  message: string;
  /** The nuance behind it, when there is any. */
  detail?: string;
}

/**
 * What one send actually did, said in a sentence.
 *
 * A send has five outcomes, not two. `applied` and `failed` are terminal;
 * `requeued` and `errored` are not, and both mean "send again" — a row that
 * transiently failed will retry, and a row whose outcome could not be written
 * down is sitting in_flight waiting to be reclaimed. `halted` means the pass
 * stopped early and everything behind that point is untouched.
 *
 * Reporting only the two terminal numbers turns every non-terminal outcome into
 * silence on an HTTP 200: an operator reads "0 sent, 0 failed" after a pass
 * that requeued eight changes and concludes the queue is idle.
 */
export function describeDrain(result: DrainResult | undefined): DrainOutcome {
  const applied = result?.applied ?? 0;
  const failed = result?.failed ?? 0;
  const requeued = result?.requeued ?? 0;
  const errored = result?.errored ?? 0;
  const exhausted = result?.exhausted ?? 0;

  const parts: string[] = [];
  if (applied) parts.push(`${applied} sent`);
  if (failed) parts.push(`${failed} failed`);
  if (requeued) parts.push(`${requeued} waiting to be sent`);
  if (errored) parts.push(`${errored} not recorded`);
  const counts = parts.join(" · ");

  // A send runs one pass per connected system, and the passes fail
  // independently — so "halted" no longer means "nothing happened". A Zitadel
  // outage stops Zitadel's pass and leaves a reachable NAS's queue moving, and
  // telling an operator nothing was sent when nine changes landed is the worse
  // half of that to get wrong.
  const halted = (result?.passes ?? []).filter((pass) => pass.halted);
  const whose = result?.halted_target ?? halted[0]?.target;
  const stalled =
    halted.length > 0
      ? halted.map((pass) => targetLabel(pass.target)).join(", ")
      : whose
        ? targetLabel(whose)
        : undefined;

  // Only when the response actually says whose pass stopped. A single-system
  // send carries no `passes`, and "undefined was skipped" is worse than the
  // reason-shaped sentence below.
  if (result?.halted && counts && stalled) {
    return {
      tone: "warning",
      message: `${counts}. ${stalled} was skipped.`,
      detail: haltDetail(result.reason, stalled),
    };
  }

  if (result?.halted) {
    switch (result.reason) {
      case "drain_in_progress":
        return {
          tone: "info",
          message: "Another send is already running.",
          detail: "This one did nothing. No change was sent twice.",
        };
      case "zitadel_offline":
        return {
          tone: "error",
          message: "Zitadel is not answering. Nothing was sent.",
          detail: "Every change is still waiting to be sent, in order.",
        };
      case "target_unreachable":
        return {
          tone: "error",
          message: `${stalled ?? "The connected system"} is not answering. Nothing was sent.`,
          detail:
            "Syndra checked once before sending anything, so nothing counts against the retry limit. Everything is still waiting to be sent.",
        };
      case "max_retries_exceeded":
        return {
          tone: "error",
          message: counts
            ? `${counts}. Stopped — Syndra has given up on one change.`
            : "Stopped — Syndra has given up on one change.",
          detail:
            "The queue is paused at that change, and nothing behind it was tried. Give or revoke the access again from the person's page to try afresh.",
        };
      default:
        return {
          tone: "error",
          message: counts ? `${counts}. Stopped early.` : "Stopped early.",
        };
    }
  }

  if (!counts) {
    return { tone: "info", message: "Nothing was waiting." };
  }

  // Given up is terminal and nobody will try again, so it outranks the "send
  // again" advice below — sending again does nothing for these rows.
  if (exhausted) {
    return {
      tone: "error",
      message: `${counts}. Syndra has given up on ${exhausted === 1 ? "one change" : `${exhausted} changes`}.`,
      detail:
        "Those changes will not be tried again; the rest of the queue kept moving. Give or revoke the access again from the person's page to try afresh.",
    };
  }

  // Neither of these is terminal, and both are invisible unless said out loud.
  if (requeued || errored) {
    return {
      tone: "warning",
      message: `${counts} — send again.`,
      detail: errored
        ? "A change reached Zitadel but Syndra could not record the outcome. Sending again settles it."
        : "Those changes hit a temporary error, and Syndra will try them again.",
    };
  }

  return { tone: failed ? "warning" : "success", message: `${counts}.` };
}

/**
 * The same send, in the vocabulary every other action reports in.
 *
 * `describeDrain` keeps its own tones because they carry a distinction the
 * five outcome words do not: `info` is "this pass was a no-op", which is
 * neither applied nor failed. The mapping is where that judgement is spent:
 *
 *  - a pass that moved changes and stopped nothing is **applied**
 *  - a pass that left work outstanding — requeued, unrecorded, or one system's
 *    pass halted while others went through — is **queued**, because that is
 *    exactly what it is: recorded here, not yet at the system
 *  - a pass that sent nothing is **failed**
 *  - a pass with nothing to do is **no change**, which is a fact about the
 *    queue and not an achievement
 *
 * Warning becomes `queued` rather than `failed` deliberately. "Send again" is
 * not a failure, and an operator who reads it as one goes looking for a broken
 * machine instead of pressing the button again.
 */
export function outcomeFromDrain(result: DrainResult | undefined): ActionOutcome {
  const { tone, message, detail } = describeDrain(result);
  const kind: OutcomeKind =
    tone === "success" ? "applied" : tone === "warning" ? "queued" : tone === "error" ? "failed" : "no_change";
  return { kind, message, detail };
}

/**
 * Why one system's pass did not run, said in the plural send's voice.
 *
 * Separate from the single-reason switch above because the sentence is
 * different when other passes DID work: the operator is not being told the
 * send failed, they are being told which part of it is still outstanding.
 */
function haltDetail(reason: string | undefined, system: string | undefined): string {
  switch (reason) {
    case "zitadel_offline":
      return "Zitadel is not answering. Its changes are still waiting to be sent, in order; the rest went through.";
    case "target_unreachable":
      return `${system ?? "The connected system"} is not answering. Its changes are still waiting to be sent, and nothing counts against the retry limit.`;
    case "max_retries_exceeded":
      return "Syndra has given up on a change there, so that queue is paused at it. Nothing behind it was tried.";
    case "drain_in_progress":
      return "Another send was already running for it.";
    default:
      return "The rest of the queue went through.";
  }
}

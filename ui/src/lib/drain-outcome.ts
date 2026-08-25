import type { ActionOutcome, OutcomeKind } from "@/lib/outcome";
import type { DrainResult } from "@/lib/queries/usePropagation";

export type DrainTone = "success" | "warning" | "error" | "info";

export interface DrainOutcome {
  tone: DrainTone;
  /** One scannable line. */
  message: string;
  /** The nuance behind it, when there is any. Rendered as the toast's description. */
  detail?: string;
}

/**
 * What one drain pass actually did, said in a sentence.
 *
 * A drain has five outcomes, not two. `applied` and `failed` are terminal;
 * `requeued` and `errored` are not, and both mean "click resume again" — a row
 * that transiently failed will retry, and a row whose outcome could not be
 * written down is sitting in_flight waiting to be reclaimed. `halted` means the
 * pass stopped early and everything behind that point is untouched.
 *
 * Reporting only the two terminal numbers turns every non-terminal outcome into
 * silence on an HTTP 200: an operator reads "0 applied, 0 failed" after a pass
 * that requeued eight writes and concludes the queue is idle. This is the whole
 * reason the deleted `DrainResultBanner` existed.
 */
export function describeDrain(result: DrainResult | undefined): DrainOutcome {
  const applied = result?.applied ?? 0;
  const failed = result?.failed ?? 0;
  const requeued = result?.requeued ?? 0;
  const errored = result?.errored ?? 0;
  const exhausted = result?.exhausted ?? 0;

  const parts: string[] = [];
  if (applied) parts.push(`${applied} applied`);
  if (failed) parts.push(`${failed} failed`);
  if (requeued) parts.push(`${requeued} still queued`);
  if (errored) parts.push(`${errored} not recorded`);
  const counts = parts.join(" · ");

  // A drain runs one pass per registered target, and the passes fail
  // independently — so "halted" no longer means "nothing happened". A Zitadel
  // outage stops Zitadel's pass and leaves a reachable NAS's queue moving, and
  // telling an operator nothing was sent when nine writes landed is the worse
  // half of that to get wrong.
  const halted = (result?.passes ?? []).filter((pass) => pass.halted);
  const whose = result?.halted_target ?? halted[0]?.target;
  const stalled = halted.length > 0 ? halted.map((pass) => pass.target).join(", ") : whose;

  // Only when the response actually says whose pass stopped. A single-target
  // drain carries no `passes`, and "undefined did not run" is worse than the
  // reason-shaped sentence below.
  if (result?.halted && counts && stalled) {
    return {
      tone: "warning",
      message: `${counts}. ${stalled} did not run.`,
      detail: haltDetail(result.reason, stalled),
    };
  }

  if (result?.halted) {
    switch (result.reason) {
      case "drain_in_progress":
        return {
          tone: "info",
          message: "Another resume is already running.",
          detail: "This one did nothing — no write was sent twice.",
        };
      case "zitadel_offline":
        return {
          tone: "error",
          message: "The identity provider is unreachable. Nothing was sent.",
          detail: "Every write stays queued and in order.",
        };
      case "target_unreachable":
        return {
          tone: "error",
          message: `${stalled ?? "That target"} is unreachable. Nothing was sent.`,
          detail:
            "Checked once before anything was claimed, so no write spent a retry on it. Everything stays queued.",
        };
      case "max_retries_exceeded":
        return {
          tone: "error",
          message: counts ? `${counts}. Stopped — a write is out of retries.` : "Stopped — a write is out of retries.",
          detail: "The queue is paused at that row. Nothing behind it was attempted.",
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

  // Out of retries is terminal and nobody will try again, so it outranks the
  // "resume again" advice below — resuming does nothing for these rows.
  if (exhausted) {
    return {
      tone: "error",
      message: `${counts}. ${exhausted === 1 ? "One write is" : `${exhausted} writes are`} out of retries.`,
      detail:
        "Those rows were given up on and the rest of the queue kept moving. They need a new request — resuming will not pick them up.",
    };
  }

  // Neither of these is terminal, and both are invisible unless said out loud.
  if (requeued || errored) {
    return {
      tone: "warning",
      message: `${counts} — resume again.`,
      detail: errored
        ? "A write reached the identity provider but Syndra couldn't record the outcome. Resuming settles it."
        : "Those writes hit a transient error and will be retried.",
    };
  }

  return { tone: failed ? "warning" : "success", message: `${counts}.` };
}

/**
 * The same drain, in the vocabulary every other action reports in.
 *
 * `describeDrain` keeps its own tones because they carry a distinction the
 * five outcome words do not: `info` is "this pass was a no-op", which is
 * neither applied nor failed. The mapping is where that judgement is spent:
 *
 *  - a pass that moved writes and stopped nothing is **applied**
 *  - a pass that left work outstanding — requeued, unrecorded, or one target's
 *    pass halted while others went through — is **queued**, because that is
 *    exactly what it is: recorded here, not yet at the target
 *  - a pass that sent nothing is **failed**
 *  - a pass with nothing to do is **no change**, which is a fact about the
 *    queue and not an achievement
 *
 * Warning becomes `queued` rather than `failed` deliberately. "Resume again"
 * is not a failure, and an operator who reads it as one goes looking for a
 * broken machine instead of pressing the button again.
 */
export function outcomeFromDrain(result: DrainResult | undefined): ActionOutcome {
  const { tone, message, detail } = describeDrain(result);
  const kind: OutcomeKind =
    tone === "success" ? "applied" : tone === "warning" ? "queued" : tone === "error" ? "failed" : "no_change";
  return { kind, message, detail };
}

/**
 * Why one target's pass did not run, said in the plural drain's voice.
 *
 * Separate from the single-reason switch above because the sentence is
 * different when other passes DID work: the operator is not being told the
 * drain failed, they are being told which part of it is still outstanding.
 */
function haltDetail(reason: string | undefined, target: string | undefined): string {
  switch (reason) {
    case "zitadel_offline":
      return "The identity provider is unreachable. Its writes stay queued and in order; the rest went through.";
    case "target_unreachable":
      return `${target ?? "That target"} is not answering. Its writes stay queued — nothing spent a retry on it.`;
    case "max_retries_exceeded":
      return "A write there is out of retries, so that queue is paused at it. Nothing behind it was attempted.";
    case "drain_in_progress":
      return "Another resume was already running for it.";
    default:
      return "The rest of the queue went through.";
  }
}

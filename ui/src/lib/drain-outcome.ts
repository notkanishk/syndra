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

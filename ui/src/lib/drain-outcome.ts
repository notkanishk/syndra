import { targetLabel } from "@/lib/nav";
import { DRAIN_UNKNOWN_NEXT, type ActionOutcome, type AutoRetry, type OutcomeKind } from "@/lib/outcome";
import type { DrainResult } from "@/lib/queries/usePropagation";

export type DrainTone = "success" | "warning" | "error" | "info";

export interface DrainOutcome {
  tone: DrainTone;
  /** One scannable line. */
  message: string;
  /** The nuance behind it, when there is any. */
  detail?: string;
  /** What is left goes on its own: no "send it" step for the reader. */
  sendsItself?: boolean;
  /** The step left, when it is not the kind's default. */
  next?: string;
}

/**
 * What one send actually did, said in a sentence.
 *
 * A send has five outcomes, not two. `applied` and `failed` are terminal;
 * `requeued` and `errored` are not, and neither needs the operator: a row
 * Zitadel didn't take is sent again on its own at its next try (each row on
 * Pending changes says when), and a row whose outcome could not be written
 * down is reclaimed the same way. `halted` means the pass stopped early and
 * everything behind that point is untouched.
 *
 * Reporting only the two terminal numbers turns every non-terminal outcome into
 * silence on an HTTP 200: an operator reads "0 sent, 0 failed" after a pass
 * that requeued eight changes and concludes the queue is idle.
 */
/** `nameOf` is the roster's name for a system (useSystemNames); the id title-cased without one. */
export function describeDrain(
  result: DrainResult | undefined,
  auto: AutoRetry = true,
  nameOf: (target: string) => string = targetLabel,
): DrainOutcome {
  const applied = result?.applied ?? 0;
  const failed = result?.failed ?? 0;
  const requeued = result?.requeued ?? 0;
  const errored = result?.errored ?? 0;
  const exhausted = result?.exhausted ?? 0;

  const parts: string[] = [];
  if (applied) parts.push(`${applied} sent`);
  if (failed) parts.push(`${failed} failed`);
  if (requeued) parts.push(`${requeued} to be sent again`);
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
      ? halted.map((pass) => nameOf(pass.target)).join(", ")
      : whose
        ? nameOf(whose)
        : undefined;

  // Only when the response actually says whose pass stopped. A single-system
  // send carries no `passes`, and "undefined was skipped" is worse than the
  // reason-shaped sentence below.
  if (result?.halted && counts && stalled) {
    // How many the skipped systems left waiting, when every halted pass said
    // (browser pass, 8 Nov). A system with nothing of its own to send is not
    // halted at all, so it is never named here.
    const left = halted.every((pass) => typeof pass.waiting === "number")
      ? halted.reduce((n, pass) => n + (pass.waiting ?? 0), 0)
      : undefined;
    // One clause per system, in the fold's order (A-22 §3.5): who got theirs, then who didn't answer.
    const wentTo = (result.passes ?? []).filter((pass) => !pass.halted && pass.applied > 0).map((pass) => nameOf(pass.target));
    const went = applied ? (wentTo.length ? `${applied} reached ${wentTo.join(" and ")}` : `${applied} sent`) : "";
    const rest = parts.filter((part) => !part.endsWith(" sent"));
    const still = left ? `its ${left}` : "its changes";
    return {
      tone: "warning",
      message: `${[went, ...rest].filter(Boolean).join(". ")}. ${stalled} didn't answer, so ${still} ${left === 1 ? "is" : "are"} still here.`,
      detail: haltDetail(result.reason, stalled, left || undefined),
    };
  }

  if (result?.halted) {
    switch (result.reason) {
      case "drain_in_progress":
        return {
          tone: "info",
          message: "Another send is already running.",
          detail: "This one did nothing, so no change was sent twice.",
        };
      case "zitadel_offline":
        return {
          tone: "error",
          message: "Zitadel is not answering. Nothing was sent.",
          detail:
            auto === null
              ? "Nothing counts against the retry limit. Everything stays in Pending changes until it reaches Zitadel."
              : auto
                ? "Nothing counts against the retry limit. Revocations, and changes Zitadel didn't take before, go again on their own once it answers; the rest wait for you to send."
                : "Nothing counts against the retry limit. This Syndra server doesn't send anything again on its own, so everything waits for you to press Send once Zitadel answers.",
        };
      case "target_unreachable":
        return {
          tone: "error",
          message: `${stalled ?? "The connected system"} is not answering. Nothing was sent.`,
          detail:
            "Syndra checked once before sending anything, so nothing counts against the retry limit. Everything is still waiting for you to send.",
        };
      // No "max_retries_exceeded": a spent row is made terminal and the pass
      // carries on (propagation/drain.go processRow), so no pass halts on one.
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
      tone: "warning",
      message: `${counts}. Syndra stopped sending ${exhausted === 1 ? "one change" : `${exhausted} changes`}.`,
      detail: "Those changes will not be tried again; the rest of the queue kept moving.",
      next: STOPPED_NEXT,
    };
  }

  // Neither of these is terminal, and both are invisible unless said out loud.
  // Neither asks anything of the operator: the background drain sends them
  // again at their next try, which each row below names.
  if (requeued || errored) {
    if (auto === null) {
      // Not known yet whether the background drain is on (N3): no promise.
      return {
        tone: "warning",
        sendsItself: true,
        message: `${counts}.`,
        detail: errored
          ? "Syndra could not record how a change went. It stays in Pending changes until it is settled."
          : `Zitadel didn't answer for ${requeued === 1 ? "one change" : `${requeued} changes`}, so ${requeued === 1 ? "it is" : "they are"} still in Pending changes.`,
        next: DRAIN_UNKNOWN_NEXT,
      };
    }
    if (!auto) {
      // The background drain is off: nothing sends these again until Send.
      return {
        tone: "warning",
        message: `${counts}.`,
        detail: errored
          ? "Syndra could not record how a change went. This Syndra server doesn't settle it on its own, so press Send on Pending changes to try again."
          : `Zitadel didn't answer for ${requeued === 1 ? "one change" : `${requeued} changes`}. This Syndra server doesn't send ${requeued === 1 ? "it" : "them"} again on its own, so ${requeued === 1 ? "it waits" : "they wait"} in Pending changes until you press Send.`,
      };
    }
    return {
      tone: "warning",
      sendsItself: true,
      message: `${counts}.`,
      detail: errored
        ? "Syndra could not record how a change went. It settles it on its own at the next try; each change below says when."
        : `Zitadel didn't answer for ${requeued === 1 ? "one change" : `${requeued} changes`}. Syndra sends ${requeued === 1 ? "it" : "them"} again on its own; each change below says when.`,
    };
  }

  // A failed row is terminal: Send won't move it, so the step is not "send it"
  // (re-audit (c): the "Waiting for you to send" link was drawn here).
  return failed
    ? { tone: "warning", message: `${counts}.`, detail: "Zitadel refused what failed, so it is not sent again.", next: STOPPED_NEXT }
    : { tone: "success", message: `${counts}.` };
}

/**
 * What to do about a change Syndra stopped sending. Its record is gone, so the
 * person's page has nothing to revoke: a revocation still held in Zitadel is
 * raised in Drift by the next check, and revoked there; a give is given again.
 */
export const STOPPED_NEXT =
  "Each is listed under Syndra stopped sending, on Pending changes. Give a role again from the person's page; a revocation shows in Drift after Syndra's next check, to revoke there.";

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
export function outcomeFromDrain(
  result: DrainResult | undefined,
  auto: AutoRetry = true,
  total?: number,
  nameOf: (target: string) => string = targetLabel,
): ActionOutcome & { word: string } {
  const { tone, message, detail, sendsItself, next } = describeDrain(result, auto, nameOf);
  const kind: OutcomeKind =
    tone === "success" ? "applied" : tone === "warning" ? "queued" : tone === "error" ? "failed" : "no_change";
  return { kind, message, detail, ...(sendsItself ? { sendsItself } : {}), ...(next ? { next } : {}), word: drainWord(result, total) };
}

/**
 * The result word of a send, one table for Pending changes and Home (A-22
 * §3.5, G29): it counts changes and says the whole when some did not go.
 * "Sent 7 of 9" when some are still here, "Sent 9" when all went, "Not sent"
 * when none did (a halted pass, or another send already running), "Nothing to
 * send" when nothing was waiting. `total` is the queue as it stood at the press.
 */
export function drainWord(result: DrainResult | undefined, total?: number): string {
  const applied = result?.applied ?? 0;
  const seen = applied + (result?.failed ?? 0) + (result?.requeued ?? 0) + (result?.errored ?? 0);
  if (result?.halted && result.reason === "drain_in_progress") return "Not sent";
  if (!result?.halted && seen === 0) return "Nothing to send";
  if (applied === 0) return "Not sent";
  const whole = Math.max(total ?? 0, seen);
  return applied < whole ? `Sent ${applied} of ${whole}` : `Sent ${applied}`;
}

/**
 * Why one system's pass did not run, said in the plural send's voice.
 *
 * Separate from the single-reason switch above because the sentence is
 * different when other passes DID work: the operator is not being told the
 * send failed, they are being told which part of it is still outstanding.
 */
function haltDetail(reason: string | undefined, system: string | undefined, waiting?: number): string {
  const its = waiting === undefined ? "Its changes are" : waiting === 1 ? "Its 1 change is" : `Its ${waiting} changes are`;
  switch (reason) {
    case "zitadel_offline":
      return `Zitadel is not answering. ${its} still waiting for you to send, in order; the rest went through.`;
    case "target_unreachable":
      return `${system ?? "The connected system"} is not answering. ${its} still waiting for you to send, and nothing counts against the retry limit.`;
    case "drain_in_progress":
      return "Another send was already running for it.";
    default:
      return "The rest of the queue went through.";
  }
}

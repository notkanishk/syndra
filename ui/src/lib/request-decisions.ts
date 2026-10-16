"use client";

import { useState } from "react";

import { formatShortDate, humanizeKey } from "@/lib/format";
import { outcomeFromError, type ActionOutcome } from "@/lib/outcome";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useDecideRequest, type AccessRequest, type DecisionResult } from "@/lib/queries/useRequests";
import { waitReasonLine } from "@/lib/wait-reason";

/**
 * What approving one request reports. The response says only whether the
 * approval is in Zitadel yet (`wait_reason`), so that is all this can claim.
 *
 * `message` is the sentence; `detail` is the one faint line the row's
 * Decision seat shows under the word (route-requests, #96).
 */
export function approvalOutcome(
  waitReason: string | undefined,
  name: string,
  role: string,
  result?: Pick<DecisionResult, "effect" | "until">,
): ActionOutcome {
  if (!waitReason) {
    const ends = result?.until ? `until ${formatShortDate(result.until)}` : "with no end date";
    return {
      kind: "applied",
      message:
        result?.effect === "extended"
          ? `Approved. ${name} keeps it ${ends}.`
          : result?.effect === "given_again"
            ? `Approved. It had ended, so ${name} has it again ${ends}.`
            : `Approved. ${name} can use it now.`,
      detail: "Sent · not yet seen in Zitadel",
    };
  }
  if (waitReason === "role_waits") {
    return {
      kind: "queued",
      message: "Approved. This role always waits for you, so it's in Pending changes.",
      detail: waitReasonLine(waitReason, role),
    };
  }
  const why = waitReasonLine(waitReason, role, new Date().toISOString());
  return {
    kind: "queued",
    message: `Approved. Not in Zitadel yet${why ? `: ${why}` : ""}. It goes when you send it.`,
    detail:
      waitReason === "halted:zitadel_offline" ? "Zitadel didn't answer when it was approved" : why,
  };
}

/** A decline never reaches Zitadel: nobody had the access yet. */
export const DECLINED: ActionOutcome = {
  kind: "no_change",
  message: "Declined",
  detail: "Nothing was sent",
};

/** What a decided row reports in its seat: the word, and what happened. */
export interface Decided {
  /** Absent for a failure: the seat then says Failed or Refused. */
  word?: string;
  outcome: ActionOutcome;
}

/**
 * Deciding requests in place, one row at a time — the queue, Home and a
 * person's Requests tab all decide through this, so an approval says the same
 * thing wherever it was made.
 *
 * `resolved` is marked before the await: `isPending` clears the moment the
 * write settles and the list has not refetched yet, and a second tap in that
 * window decided the same request twice. A failure puts the row back.
 */
export function useRequestDecisions() {
  const decide = useDecideRequest();
  const resolver = useNameResolver();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
  const [outcomes, setOutcomes] = useState<Record<string, Decided>>({});

  async function act(entry: AccessRequest, next: "approved" | "rejected") {
    setResolved((prev) => new Set(prev).add(entry.id));
    try {
      const result = await decide.mutateAsync({ id: entry.id, status: next });
      const decided: Decided =
        next === "approved"
          ? {
              word: "Approved",
              outcome: approvalOutcome(
                result?.wait_reason,
                resolver.resolveUser(entry.requester_id).value?.display_name || "They",
                resolver.resolveRole(entry.project_id, entry.role_key).value?.display_name ||
                  humanizeKey(entry.role_key),
                result,
              ),
            }
          : { word: "Declined", outcome: DECLINED };
      setOutcomes((prev) => ({ ...prev, [entry.id]: decided }));
    } catch (error) {
      setResolved((prev) => {
        const copy = new Set(prev);
        copy.delete(entry.id);
        return copy;
      });
      setOutcomes((prev) => ({ ...prev, [entry.id]: { outcome: outcomeFromError(error) } }));
    }
  }

  return { act, resolved, outcomes, isPending: decide.isPending };
}

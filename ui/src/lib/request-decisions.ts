"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { KEPT, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { request } from "@/lib/api-client";
import { formatShortDate } from "@/lib/format";
import { personOr, roleName } from "@/lib/names";
import { FORECAST, forecastFor, outcomeFromError, SEND_IT, type ActionOutcome } from "@/lib/outcome";
import { useRoleWaits } from "@/lib/queries/useRoleWaits";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useDecideRequest, type AccessRequest, type DecisionResult } from "@/lib/queries/useRequests";
import { FLASH_MS } from "@/lib/motion";
import { RETRYING, retryingOutcome } from "@/lib/retry";
import { waitReasonLine } from "@/lib/wait-reason";

/**
 * What approving one request reports. The response says only whether the
 * approval is in Zitadel yet (`wait_reason`), so that is all this can claim.
 * A send Zitadel didn't take is never "Sent": it says when Syndra sends it
 * again, and which try (`retry`, lib/retry.ts).
 *
 * `message` is the sentence; `detail` is the one faint line the row's
 * Decision seat shows under the word (route-requests, #96).
 */
export function approvalOutcome(
  waitReason: string | undefined,
  name: string,
  role: string,
  result?: Pick<DecisionResult, "effect" | "until" | "retry" | "rule_waiting">,
  alsoGave: RuleGift[] = [],
): ActionOutcome {
  const outcome = approvalOnly(waitReason, name, role, result);
  const rules = ruleGiftLine(alsoGave);
  const withRules = rules ? { ...outcome, message: `${outcome.message} ${rules}` } : outcome;
  return withRuleWaits(withRules, result?.rule_waiting);
}

/**
 * A role an automatic rule gave with the act that waits for you (re-audit
 * R12): said apart from the act's own outcome, which is never blamed for it,
 * and the result ends on Send. The act itself may have gone straight away.
 */
export function withRuleWaits(outcome: ActionOutcome, ruleWaiting: number | undefined): ActionOutcome {
  if (!ruleWaiting) return outcome;
  const line =
    ruleWaiting === 1
      ? "One role an automatic rule also gives always waits for you, so it is in Pending changes."
      : `${ruleWaiting} roles automatic rules also give always wait for you, so they are in Pending changes.`;
  // Already waiting for Send: the link says it once.
  if (outcome.kind === "queued" && !outcome.sendsItself) return { ...outcome, message: `${outcome.message} ${line}` };
  return { ...outcome, kind: "queued", sendsItself: undefined, message: `${outcome.message} ${line}`, next: SEND_IT };
}

/** A role an automatic rule also gave with the approval, by name. */
export interface RuleGift {
  role: string;
  project: string;
}

/**
 * "An automatic rule also gave them Bench in Workshop." The approval's whole
 * effect, not only the role asked for (coherence audit C2): the rules fire on
 * the same write, under the same delivery.
 */
export function ruleGiftLine(gifts: RuleGift[]): string {
  if (gifts.length === 0) return "";
  const names = gifts.map((g) => `${g.role} in ${g.project}`);
  const list = names.length === 1 ? names[0] : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
  return `${gifts.length === 1 ? "An automatic rule" : "Automatic rules"} also gave them ${list}.`;
}

/**
 * What the automatic rules would also give with this approval, asked BEFORE
 * the approval is written (afterwards they already hold it, and the answer is
 * empty). The backend's own closure (`GET /users/{id}/rule-preview`), never one
 * walked here. A failed read says nothing rather than something wrong.
 */
export async function previewRuleGifts(
  entry: Pick<AccessRequest, "requester_id" | "project_id" | "role_key">,
): Promise<Array<{ project_id: string; role_key: string }>> {
  const query = new URLSearchParams({ project_id: entry.project_id, role_key: entry.role_key });
  try {
    const data = await request<{ rows?: Array<{ project_id: string; role_key: string }> }>(
      `/users/${encodeURIComponent(entry.requester_id)}/rule-preview?${query}`,
    );
    return data?.rows ?? [];
  } catch {
    return [];
  }
}

function approvalOnly(
  waitReason: string | undefined,
  name: string,
  role: string,
  result?: Pick<DecisionResult, "effect" | "until" | "retry" | "rule_waiting">,
): ActionOutcome {
  if (waitReason === RETRYING) return retryingOutcome("Approved", result?.retry);
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

/**
 * Approving a request to keep a role, once the end has moved: the end it has now
 * and the one it had. Nothing is sent to Zitadel for an end date, so there is no
 * verdict line to give.
 */
export function extendedOutcome(
  entry: Pick<AccessRequest, "current_until" | "until">,
  result?: Pick<DecisionResult, "until">,
): ActionOutcome {
  const now = result?.until ?? entry.until;
  const text = !now
    ? "Extended"
    : entry.current_until
      ? `Now ends ${formatShortDate(now)}, not ${formatShortDate(entry.current_until)}`
      : `Now ends ${formatShortDate(now)}`;
  return { kind: "applied", message: text, detail: `${text}.` };
}

/** Declining a request to keep a role: nothing was sent, and the end hasn't moved. */
export function declinedKeepOutcome(entry: Pick<AccessRequest, "current_until">): ActionOutcome {
  return {
    kind: "no_change",
    message: "Declined",
    detail: entry.current_until
      ? `Nothing was sent · it still ends ${formatShortDate(entry.current_until)}.`
      : "Nothing was sent",
  };
}

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
export function useRequestDecisions({ pauseBeat = false }: { pauseBeat?: boolean } = {}) {
  const decide = useDecideRequest();
  const resolver = useNameResolver();
  const roleWaits = useRoleWaits();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
  const [outcomes, setOutcomes] = useState<Record<string, Decided>>({});
  // The first 100ms (A-19 P2): what Syndra WILL do, in the seat, while the write is in flight.
  // Words only. It is cleared by the true word or by the error; nothing is rolled back because
  // nothing was changed on screen.
  const [forecasts, setForecasts] = useState<Record<string, string>>({});
  const forecast = (id: string, word: string | null) =>
    setForecasts((prev) => {
      const { [id]: _gone, ...rest } = prev;
      return word ? { ...rest, [id]: word } : rest;
    });
  // Approving can cancel a removal still waiting for you to send. A row has no
  // sheet of its own, so the step opens a small sheet holding only itself.
  const [deciding, setDeciding] = useState<AccessRequest | null>(null);
  const first = (resolver.resolveUser(deciding?.requester_id ?? "").value?.display_name ?? "").trim().split(/\s+/)[0];
  const step = useCancelStep({
    host: first ? `Approve ${first}'s request` : "Approve this request",
    goAhead: "approve",
    standalone: true,
  });

  async function act(entry: AccessRequest, next: "approved" | "rejected") {
    setResolved((prev) => new Set(prev).add(entry.id));
    setDeciding(entry);
    // A role that waits forecasts the wait, not the approval (A-20 P8).
    forecast(
      entry.id,
      next === "rejected"
        ? FORECAST.decline
        : forecastFor(entry.kind === "extend" ? "extend" : "approve", roleWaits(entry.project_id, entry.role_key)),
    );
    // Asked before the write: once approved, the rules' roles are already held.
    const gifts = next === "approved" && entry.kind !== "extend" ? previewRuleGifts(entry) : Promise.resolve([]);
    try {
      const result = await step.run(() => decide.mutateAsync({ id: entry.id, status: next }));
      if (result === KEPT) {
        forecast(entry.id, null);
        // Kept: the request is still waiting, in its seat, and nothing was decided.
        setResolved((prev) => {
          const copy = new Set(prev);
          copy.delete(entry.id);
          return copy;
        });
        return;
      }
      const keeping = entry.kind === "extend";
      let decided: Decided;
      if (next === "rejected") {
        decided = { word: "Declined", outcome: keeping ? declinedKeepOutcome(entry) : DECLINED };
      } else if (keeping && !result?.wait_reason) {
        decided = { word: "Extended", outcome: extendedOutcome(entry, result) };
      } else {
        decided = {
          word: "Approved",
          outcome: approvalOutcome(
            result?.wait_reason,
            personOr(resolver.resolveUser(entry.requester_id).value?.display_name, true),
            roleName(resolver.resolveRole(entry.project_id, entry.role_key).value?.display_name, entry.role_key),
            result,
            (await gifts).map((row) => ({
              role: roleName(resolver.resolveRole(row.project_id, row.role_key).value?.display_name, row.role_key),
              project: resolver.resolveProject(row.project_id).value?.name || "its project",
            })),
          ),
        };
      }
      setOutcomes((prev) => ({ ...prev, [entry.id]: decided }));
      forecast(entry.id, null);
    } catch (error) {
      forecast(entry.id, null);
      setResolved((prev) => {
        const copy = new Set(prev);
        copy.delete(entry.id);
        return copy;
      });
      setOutcomes((prev) => ({ ...prev, [entry.id]: { outcome: outcomeFromError(error) } }));
    }
  }

  /** Rows decided somewhere else (the bulk dialog) report in the same seat, and hold the same beat. */
  function settleAll(decided: Array<[AccessRequest, Decided]>) {
    setResolved((prev) => new Set(Array.from(prev).concat(decided.map(([entry]) => entry.id))));
    setOutcomes((prev) => ({ ...prev, ...Object.fromEntries(decided.map(([entry, d]) => [entry.id, d])) }));
  }

  // Motion 5b's beat: a decided row keeps its place with its outcome for one
  // flash, so the operator sees what the tap did before the list rearranges.
  // `staying` is every row still in it, the write in flight included. It is
  // held for the whole of that, so the read that drops the row (the refetch
  // lands in tens of milliseconds) cannot take the outcome with it.
  const [over, setOver] = useState<Set<string>>(new Set());
  const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  useEffect(() => {
    if (pauseBeat) return;
    for (const [id, decided] of Object.entries(outcomes)) {
      if (!decided.word || !resolved.has(id) || timers.current.has(id)) continue;
      timers.current.set(
        id,
        setTimeout(() => setOver((prev) => new Set(prev).add(id)), FLASH_MS),
      );
    }
  }, [outcomes, resolved, pauseBeat]);
  useEffect(() => {
    const pending = timers.current;
    return () => {
      pending.forEach(clearTimeout);
      pending.clear();
    };
  }, []);
  const staying = useMemo(() => new Set(Array.from(resolved).filter((id) => !over.has(id))), [resolved, over]);

  /** Render once beside the rows: the step for an approval that would cancel something. */
  return { act, settleAll, resolved, staying, outcomes, forecasts, isPending: decide.isPending, cancelStep: step.element };
}

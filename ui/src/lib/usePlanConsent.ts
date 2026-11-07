"use client";

import { useQueries, useQuery, type QueryClient } from "@tanstack/react-query";

import { ApiError, request, WouldCancelQueued } from "@/lib/api-client";
import type { QueuedChange } from "@/lib/cancel-confirm";

/**
 * A single act asks the planner before the press (architecture deepening
 * decision 9, A-21 §3). One read, `GET /users/{id}/plan`, returns the act's
 * Change: the planner's words for it, what the automatic rules add (the one
 * source of those sentences, A-21 §4), whether it goes now or waits, and the
 * queued changes it would cancel, with the fingerprint and cancel ids the
 * write carries back. Give, Revoke, Approve and Manage bundles read it; it
 * replaces the rule preview, `previewRuleGifts` and Manage bundles' what-if.
 */

export type PlanAction =
  | { op: "give"; userId: string; projectId: string; roleKey: string }
  | { op: "remove"; userId: string; grantId: string }
  | { op: "approve"; userId: string; requestId: string }
  | { op: "assign_bundle" | "remove_bundle"; userId: string; bundleId: string };

/** The act's Change as a sheet reads it (handlers.ActPlan). */
export interface ActPlan {
  /** apply, no_change or blocked. */
  effect: "apply" | "no_change" | "blocked";
  /** Why it doesn't go ahead: held, not_offered:…, departed:…, catalogue_unread, … */
  refusal?: string;
  detail: string;
  /** The planner's sentences after `detail`, the rules' among them, without the cancels line. */
  said: string[];
  /** A refusal's step. */
  consequence?: string;
  /** "Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab." */
  rules: string[];
  /** straight_away, waits_for_you, or none (it only cancels a waiting change). */
  delivery?: "straight_away" | "waits_for_you" | "none";
  waits: string[];
  cancels: QueuedChange[];
  fingerprint: string;
  cancel_ids: string[];
}

/** What a write carries of the plan it was shown. */
export interface PlanSeen {
  fingerprint: string;
  cancel_ids: string[];
}

export const seenOf = (plan: ActPlan | undefined): PlanSeen | undefined =>
  plan ? { fingerprint: plan.fingerprint, cancel_ids: plan.cancel_ids } : undefined;

/** The query string a DELETE carries the plan in. */
export function seenQuery(seen: PlanSeen | undefined): string {
  if (!seen) return "";
  const q = new URLSearchParams({ fingerprint: seen.fingerprint });
  if (seen.cancel_ids.length) q.set("cancel_ids", seen.cancel_ids.join(","));
  return q.toString();
}

/** Under ["users", id], so every access change refreshes it. */
export function planKey(action: PlanAction) {
  const { op, userId, ...rest } = action;
  return ["users", userId, "plan", op, ...Object.values(rest)] as const;
}

function planURL(action: PlanAction): string {
  const q = new URLSearchParams({ op: action.op });
  if (action.op === "give") {
    q.set("project_id", action.projectId);
    q.set("role_key", action.roleKey);
  } else if (action.op === "remove") q.set("grant_id", action.grantId);
  else if (action.op === "approve") q.set("request_id", action.requestId);
  else q.set("bundle_id", action.bundleId);
  return `/users/${encodeURIComponent(action.userId)}/plan?${q}`;
}

function ready(action: PlanAction | null): action is PlanAction {
  if (!action?.userId) return false;
  switch (action.op) {
    case "give":
      return Boolean(action.projectId && action.roleKey);
    case "remove":
      return Boolean(action.grantId);
    case "approve":
      return Boolean(action.requestId);
    default:
      return Boolean(action.bundleId);
  }
}

// Stands in for no act yet: never read (`enabled` is false).
const NO_ACTION: PlanAction = { op: "give", userId: "", projectId: "", roleKey: "" };

const planQuery = (action: PlanAction) => ({
  queryKey: planKey(action),
  queryFn: () => request<ActPlan>(planURL(action)),
});

/** The plan for the act now, read once more (the press of a row with no sheet). */
export function fetchPlan(client: QueryClient, action: PlanAction): Promise<ActPlan> {
  return client.fetchQuery({ ...planQuery(action), staleTime: 0 });
}

/**
 * The write was refused because the person's access moved since the sheet
 * read its plan (409 PLAN_STALE), or, as the second safety, because it would
 * have cancelled a change nobody was shown (409 WOULD_CANCEL_QUEUED). Either
 * way nothing was written and the plan is read again.
 */
export function isStale(error: unknown): boolean {
  return error instanceof WouldCancelQueued || (error instanceof ApiError && error.code === "PLAN_STALE");
}

/** What the sheet is told while the plan is read, and when it can't be. */
export function planWords(doing: string) {
  return {
    checking: `Checking what ${doing} does.`,
    held: `Held until Syndra can read what ${doing} does.`,
    unread: `Syndra couldn't read what ${doing} does, so it can't show what else this would give or cancel. Try again to read it.`,
  };
}

/**
 * The plan and the hold that follows from it: `hold` is the sentence under a
 * disabled button while the plan is read or when it can't be, undefined when
 * the act may go ahead. `doing` names the act in the hold ("giving it",
 * "approving").
 */
export function usePlanConsent(action: PlanAction | null, doing: string, enabled = true) {
  const plan = useQuery({ ...planQuery(action ?? NO_ACTION), enabled: enabled && ready(action) });
  return consentOf(plan, doing);
}

/** One consent per staged act (Manage bundles). */
export function usePlanConsents(actions: PlanAction[], doing: string) {
  const plans = useQueries({ queries: actions.map((a) => ({ ...planQuery(a), enabled: ready(a) })) });
  return plans.map((plan) => consentOf(plan, doing));
}

type PlanRead = { data?: ActPlan; isLoading: boolean; isError: boolean; isFetching: boolean; refetch: () => unknown };

function consentOf(plan: PlanRead, doing: string) {
  // A retry after a failure keeps the query in error until it answers: it is
  // still a read in progress, so it is "checking", not "couldn't read".
  const checking = plan.isLoading || (plan.isError && plan.isFetching);
  const failed = plan.isError && !plan.isFetching;
  const words = planWords(doing);
  return {
    plan: plan.data,
    rules: plan.data?.rules ?? [],
    cancels: plan.data?.cancels ?? [],
    seen: seenOf(plan.data),
    hold: checking ? words.checking : failed ? words.held : undefined,
    checking,
    failed,
    unread: words.unread,
    retry: () => void plan.refetch(),
  };
}

export type PlanConsent = ReturnType<typeof consentOf>;

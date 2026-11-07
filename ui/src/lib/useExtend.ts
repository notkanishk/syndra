"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { KEPT, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { EXTEND_DAYS, outcomeFromError, outcomeFromExtend, type ActionOutcome } from "@/lib/outcome";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useCreateGrant } from "@/lib/queries/useUsers";
import { fetchPlan, isStale, seenOf } from "@/lib/usePlanConsent";

/**
 * Extend on a row (Expiring access, Home, a person's page), through the
 * planner (architecture deepening 1.2a). A ledger edit: Zitadel has no end
 * dates, so nothing is sent and nothing waits. The press reads the plan, so a
 * refusal ("Already ends later than this would make it.") is said without a
 * write; an extension that would cancel a removal still waiting pushes the
 * cancel step at once (`step.confirm`); the write carries the plan's
 * fingerprint and cancel ids. Their access moving between the read and the
 * write (409 PLAN_STALE) writes nothing and says so, with the button still
 * there to press again.
 *
 * `press` resolves with the outcome to show, or null when the operator kept
 * the waiting change. `step.element` is the small sheet the step opens in.
 */
export function useExtend(userId: string, projectId: string, roleKey: string, reason: string, leaves?: string) {
  const client = useQueryClient();
  const create = useCreateGrant(userId);
  const resolver = useNameResolver();
  const step = useCancelStep({ host: "Extend this access", goAhead: "extend", standalone: true });
  const [reading, setReading] = useState(false);

  async function press(): Promise<ActionOutcome | null> {
    setReading(true);
    try {
      const plan = await fetchPlan(client, { op: "extend", userId, projectId, roleKey, days: EXTEND_DAYS });
      setReading(false);
      if (plan.effect !== "apply") {
        return {
          kind: "refused",
          // Said here: a seat doesn't add it, and a block doesn't say it twice.
          message: `${plan.detail} Nothing was changed.`,
          // Already later: nothing is left to do. Anything else names its own step, or reads the row again.
          next: plan.consequence || (plan.refusal === "runs_past" ? "Nothing for you to do." : "Reload the page to see what they have now."),
        };
      }
      const result = await step.confirm(plan.cancels, () =>
        create.mutateAsync({
          project_id: projectId,
          role_key: roleKey,
          reason,
          duration_days: EXTEND_DAYS,
          extend: true,
          ...seenOf(plan),
        }),
      );
      if (result === KEPT) return null;
      return outcomeFromExtend(EXTEND_DAYS, plan.detail, leaves);
    } catch (error) {
      setReading(false);
      if (isStale(error)) {
        const first = resolver.resolveUser(userId).value?.display_name?.trim().split(/\s+/)[0];
        return {
          kind: "refused",
          message: `Nothing was changed. ${first ? `${first}'s` : "Their"} access moved while you looked.`,
          next: "Press Extend again to read it afresh.",
        };
      }
      return outcomeFromError(error);
    }
  }

  return {
    press,
    /** The plan is being read or the write is in flight: the button is busy. */
    busy: reading || create.isPending,
    /** The write is in flight: the seat says what Syndra will do. */
    writing: create.isPending,
    step,
  };
}

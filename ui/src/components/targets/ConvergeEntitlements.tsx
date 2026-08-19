"use client";

import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import {
  useApplyEntitlements,
  useRehearseEntitlements,
  type EntitlementApplyResult,
  type EntitlementPlan,
} from "@/lib/queries/useEntitlements";
import { targetLabel } from "@/lib/nav";

/**
 * Bringing a cohort's accounts back in line with what their roles say
 * (9.3–9.6; design §23).
 *
 * The same rehearse-then-apply shape as every other planned change in the
 * product, and deliberately the same COMPONENT: an operator who has read a
 * bulk-grant plan knows where the button is here. What differs is only what the
 * plan is about — a resolved desired-state set per subject rather than a grant.
 *
 * Two properties the endpoint has that no other planned surface does, and both
 * are rendered rather than smoothed over:
 *
 *   provisional — the plan was computed against the add-on's last-known state
 *                 because the target was unreachable. It stays applicable,
 *                 labelled with the age of that state. This differs on purpose
 *                 from the unmanaged inventory, where a stale read BLOCKS
 *                 adoption: adopting binds an identity irreversibly, applying
 *                 only joins a queue an operator can still inspect.
 *   truncated   — the add-on's read hit its cap, so an absence in it is not an
 *                 absence on the target.
 *
 * The apply carries the plan id and never the original submission, which is the
 * whole mechanism: what gets queued is what was on screen.
 */
export function ConvergeEntitlements({
  target,
  subjectIds,
  label,
  onClose,
}: {
  target: string;
  /** Who to converge. Always an explicit list — never "everybody". */
  subjectIds: string[];
  /** What this cohort IS, for the lede. "everybody holding Trained operator". */
  label: string;
  onClose: () => void;
}) {
  const rehearse = useRehearseEntitlements(target);
  const apply = useApplyEntitlements(target);
  const [plan, setPlan] = useState<EntitlementPlan | null>(null);

  return (
    <>
      <RehearsalDialog
        title={`Bring accounts in line on ${targetLabel(target)}`}
        lede={`Reads what ${label} should have on ${targetLabel(target)} and what they have, and queues the difference. Nothing reaches the target until somebody resumes the queue.`}
        noun={["account", "accounts"]}
        onRehearse={async (acknowledgeScope) => {
          const result = await rehearse.mutateAsync({ subjectIds, acknowledgeScope });
          setPlan(result);
          return asBulkPlan(result);
        }}
        onApply={async (planId) => {
          const result = await apply.mutateAsync({ planId, subjectIds });
          return applyAsBulkPlan(plan, result);
        }}
        onClose={onClose}
      />
      {/* Outside the dialog's own body so it survives every step of it: the age
          of the state a provisional plan was computed against is exactly as
          relevant on the review step as on the result. */}
      {plan && <PlanProvenance plan={plan} />}
    </>
  );
}

/**
 * The age of the state this plan was computed against, and whether the read was
 * complete.
 *
 * `provisional: true` with no number is a label nobody can act on — it is the
 * difference between "computed against last-known state" and "computed against
 * state from fourteen minutes ago", and only the second one lets an operator
 * decide whether that matters.
 */
function PlanProvenance({ plan }: { plan: EntitlementPlan }) {
  return (
    // Above the page, below a dialog. It sat at z-60 — the highest number in
    // the product — which put a passive freshness read on top of the scrim of
    // any dialog the operator opened, including the ones asking them to type a
    // name before something irreversible happens.
    //
    // The bottom offset clears whatever navigation occupies, which is nothing
    // on a rail and a tab bar on a phone. Reading the token rather than
    // guessing a number is what stops this dock and the tab bar disagreeing
    // the next time either one changes height.
    <div
      className="pointer-events-none fixed inset-x-0 z-30 flex justify-center px-6"
      style={{ bottom: "calc(var(--touch-nav-height) + 24px)" }}
    >
      <div className="pointer-events-auto rounded-pill border border-line bg-surface-1 px-4 py-2 shadow-popover">
        <ReadFreshness
          subject="This plan's view of the target"
          state={{
            readAt: plan.state_read_at,
            current: !plan.provisional,
            truncated: plan.truncated,
          }}
        />
      </div>
    </div>
  );
}

/**
 * The entitlement plan in the shape the shared dialog reads.
 *
 * A translation rather than a second endpoint, because the two speak almost the
 * same language already — and the one field that differs is the one that
 * matters: `succeeded` is always zero here, so it stays zero.
 */
function asBulkPlan(plan: EntitlementPlan): BulkPlan {
  return {
    op: "converge_entitlements" as BulkPlan["op"],
    plan_id: plan.plan_id,
    applied: plan.applied,
    outcomes: plan.outcomes.map(
      (outcome): BulkOutcome => ({
        user_id: outcome.user_id,
        name: outcome.name ?? "",
        email: outcome.email ?? "",
        effect: outcome.effect,
        detail: outcome.detail,
        consequence: outcome.consequence,
      }),
    ),
    summary: {
      total: plan.summary.total,
      apply: plan.summary.apply,
      // Rows needing nothing are COUNTED, not hidden. "This changes less than
      // you think" is the most useful thing a plan can say, and a screen that
      // filtered them would leave an operator wondering where they went.
      no_change: plan.summary.no_change,
      blocked: plan.summary.blocked,
      failed: plan.summary.failed,
      succeeded: 0,
      queued: plan.summary.queued,
    },
  };
}

/**
 * And the result, which is a queue receipt rather than a report of work done.
 *
 * The dialog's result copy reads `succeeded` and `queued`. This endpoint's
 * `succeeded` is always zero by construction — present precisely so a client
 * cannot default it — so the word "done" and the tick never appear.
 */
function applyAsBulkPlan(plan: EntitlementPlan | null, result: EntitlementApplyResult): BulkPlan {
  const queuedIDs = new Set(result.queued.map((q) => q.subject_id));
  const outcomes = (plan?.outcomes ?? []).map(
    (outcome): BulkOutcome => ({
      user_id: outcome.user_id,
      name: outcome.name ?? "",
      email: outcome.email ?? "",
      effect: queuedIDs.has(outcome.user_id) ? "queued" : outcome.effect,
      detail: outcome.detail,
      consequence: outcome.consequence,
    }),
  );

  return {
    op: "converge_entitlements" as BulkPlan["op"],
    applied: true,
    outcomes,
    summary: {
      total: result.summary.total,
      apply: 0,
      no_change: result.summary.no_change,
      blocked: result.summary.blocked,
      failed: 0,
      succeeded: result.summary.succeeded,
      queued: result.summary.queued,
    },
  };
}

/**
 * The button that opens it, wherever a cohort is already on screen.
 *
 * Takes the cohort rather than fetching one: the surfaces that can honestly
 * produce a list of subjects are the ones that are already showing it, and a
 * control that assembled its own would be assembling one nobody reviewed.
 */
export function ConvergeButton({
  target,
  subjectIds,
  label,
  disabled,
  disabledReason,
}: {
  target: string;
  subjectIds: string[];
  label: string;
  disabled?: boolean;
  disabledReason?: string;
}) {
  const [open, setOpen] = useState(false);

  if (disabled) {
    // The reason as text, never a tooltip — the same rule the inventory's
    // blocked adoption follows.
    return <span className="text-[13px] text-faint">{disabledReason}</span>;
  }

  return (
    <>
      <Button variant="ghost" size="sm" onClick={() => setOpen(true)}>
        Bring accounts in line
      </Button>
      {open && (
        <ConvergeEntitlements
          target={target}
          subjectIds={subjectIds}
          label={label}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}

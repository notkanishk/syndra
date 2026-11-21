"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";

import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { ToBody } from "@/components/ui/Modal";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { whyLine } from "@/components/results/copy";
import { ApiError } from "@/lib/api-client";
import { TARGET_NOT_READ, type Answered, notRead } from "@/lib/queries/read";
import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import {
  useApplyEntitlements,
  useRehearseEntitlements,
  type EntitlementApplyResult,
  type EntitlementPlan,
} from "@/lib/queries/useEntitlements";
import { TINT_MS, useExitPhase } from "@/lib/motion";
import dynamic from "next/dynamic";

// Opened on demand, so it stays out of the page's first load.
const RehearsalDialog = dynamic(() => import("@/components/ui/RehearsalDialog").then((m) => m.RehearsalDialog));

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
 *   a kept copy — the plan was computed against the add-on's kept copy (`read.state` kept)
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
 *
 * `open` is false once the dialog is closed and the component is kept mounted
 * for the one thing that outlives it: the dock under it tints out rather than
 * vanishing with the dialog.
 */
export function ConvergeEntitlements({
  target,
  subjectIds,
  label,
  onClose,
  open = true,
}: {
  target: string;
  /** Who to converge. Always an explicit list — never "everybody". */
  subjectIds: string[];
  /** What this cohort IS, for the lede. "everybody holding Trained operator". */
  label: string;
  onClose: () => void;
  open?: boolean;
}) {
  const nameOf = useSystemNames();
  const rehearse = useRehearseEntitlements(target);
  const apply = useApplyEntitlements(target);
  // The plan as the read it is: computed against the system now, or against its kept copy, dated.
  const [planRead, setPlan] = useState<PlanRead | null>(null);
  const plan = planRead?.data ?? null;
  // What leaves is the plan it was showing; a dialog opened again starts with none.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setPlan(null);
  }
  const [lastPlan, setLastPlan] = useState(planRead);
  if (planRead && planRead !== lastPlan) setLastPlan(planRead);
  const dock = useExitPhase(open && plan !== null, TINT_MS);

  return (
    <>
      {open && (
      <RehearsalDialog
        title={`Check for differences on ${nameOf(target)}`}
        lede={`Compares what ${label} should have on ${nameOf(target)} with what they have, and records the differences as changes. Pending changes lists each one until it reaches ${nameOf(target)}.`}
        noun={["account", "accounts"]}
        system={nameOf(target)}
        onRehearse={async (acknowledgeScope) => {
          const result = await rehearse.mutateAsync({ subjectIds, acknowledgeScope });
          // Nothing to compute against (design §13.3): the why is the dialog's refusal, never an empty plan.
          if (notRead(result)) {
            throw new ApiError(503, { error: TARGET_NOT_READ, message: whyLine(result.why, nameOf(target), { audience: "operator", person: "" }) });
          }
          setPlan(result);
          return asBulkPlan(result.data);
        }}
        onApply={async (planId) => {
          const result = await apply.mutateAsync({ planId, subjectIds });
          return applyAsBulkPlan(plan, result);
        }}
        onClose={onClose}
      />
      )}
      {/* Outside the dialog's own body so it survives every step of it: the age
          of the state a provisional plan was computed against is exactly as
          relevant on the review step as on the result. */}
      {dock.rendered && lastPlan && <PlanProvenance read={lastPlan} leaving={dock.leaving} />}
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
function PlanProvenance({ read, leaving }: { read: PlanRead; leaving: boolean }) {
  return (
    // Above the page, below a dialog. It once sat above every dialog, the highest step in
    // the product — which put a passive freshness read on top of the scrim of
    // any dialog the operator opened, including the ones asking them to type a
    // name before something irreversible happens.
    //
    // The bottom offset clears whatever navigation occupies, which is nothing
    // on a rail and a tab bar on a phone. Reading the token rather than
    // guessing a number is what stops this dock and the tab bar disagreeing
    // the next time either one changes height.
    <ToBody>
    <div
      className="pointer-events-none fixed inset-x-0 z-(--layer-pill) flex justify-center px-6"
      style={{ bottom: "calc(var(--touch-nav-height) + 24px)" }}
    >
      {/* The dock lifts into place, 6px and no scale, and tints out on leave. The
          transform is on the pill and not on the `fixed` parent, which would drag
          it off its own edge. */}
      <div
        aria-hidden={leaving ? true : undefined}
        className={`${leaving ? "tint-out pointer-events-none" : "lift-in pointer-events-auto"} rounded-pill border border-line bg-surface-1 px-4 py-2 shadow-popover`}
      >
        <ReadFreshness
          subject="What this preview is based on"
          read={read}
          truncated={read.data.truncated}
        />
      </div>
    </div>
    </ToBody>
  );
}

/**
 * The entitlement plan in the shape the shared dialog reads.
 *
 * A translation rather than a second endpoint, because the two speak almost the
 * same language already — and the one field that differs is the one that
 * matters: `succeeded` is always zero here, so it stays zero.
 */
/** A plan that has data: computed now, or against the add-on's kept copy. */
type PlanRead = Exclude<Answered<EntitlementPlan>, { state: "none" }>;

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
  const queued = new Map(result.queued.map((q) => [q.subject_id, q]));
  const outcomes = (plan?.outcomes ?? []).map(
    (outcome): BulkOutcome => ({
      user_id: outcome.user_id,
      name: outcome.name ?? "",
      email: outcome.email ?? "",
      effect: queued.has(outcome.user_id) ? "queued" : outcome.effect,
      sends_on_its_own: queued.get(outcome.user_id)?.sends_on_its_own,
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
}: {
  target: string;
  subjectIds: string[];
  label: string;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        Check for differences
      </Button>
      <ConvergeEntitlements
        open={open}
        target={target}
        subjectIds={subjectIds}
        label={label}
        onClose={() => setOpen(false)}
      />
    </>
  );
}

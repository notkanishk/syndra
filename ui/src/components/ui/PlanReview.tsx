"use client";

import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import { PLAN_EFFECT_LABEL, PLAN_EFFECT_TONE } from "@/lib/outcome";

/**
 * The rehearsal, rendered.
 *
 * Every bulk operation in the product — granting a role to a cohort, resolving
 * a drift queue, deciding a batch of requests — computes the same plan shape on
 * the server and shows it through this one component. That is deliberate: an
 * operator should not have to learn what "will change" looks like separately on
 * each screen, and a preview drawn by different code from the write it previews
 * is a preview of nothing.
 *
 * Rows are never identified by id. A plan an operator cannot read is a plan
 * they will approve without reading.
 */

// The six words live in lib/outcome, shared with every other surface that
// reports what happened. They were declared here first, privately, which is
// how a plan and the row that ran it could have come to describe the same
// result in two vocabularies. `queued` keeps the warning tone rather than the
// accent one — recorded here, not yet at the target.

export function PlanReview({ plan }: { plan: BulkPlan | null }) {
  if (!plan) return null;

  // `?? []`, as everywhere else that reads a list off a payload. A plan with no
  // outcomes array means the backend is wrong, and the right failure for that
  // is a plan showing no rows beside its summary — not an error boundary
  // blanking the screen an operator is standing on halfway through approving a
  // change. This crashed the page the first time a payload arrived short.
  //
  // The two empties are told apart. An ABSENT array is a payload that arrived
  // short; an EMPTY one is a change that reaches nobody, which on the mapping
  // surfaces is an ordinary act — a definition written before anybody holds the
  // role. Rendering the same sentence for both would tell an operator their
  // perfectly good plan was broken.
  const missing = plan.outcomes === undefined || plan.outcomes === null;
  const outcomes = plan.outcomes ?? [];

  return (
    <div className="px-6">
      <div className="max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong">
        {missing ? (
          <p className="px-4 py-3 text-[13.5px] text-faint">
            The list of people did not load. The totals under the buttons are all that came back.
          </p>
        ) : outcomes.length === 0 ? (
          <p className="px-4 py-3 text-[13.5px] text-muted">
            Nobody holds this role yet, so nobody&apos;s access changes today. This only changes
            what the role gives to people who hold it later.
          </p>
        ) : (
          outcomes.map((outcome) => <PlanRow key={outcome.user_id} outcome={outcome} />)
        )}
      </div>
    </div>
  );
}

function PlanRow({ outcome }: { outcome: BulkOutcome }) {
  const label = PLAN_EFFECT_LABEL[outcome.effect] ?? PLAN_EFFECT_LABEL.no_change;
  const tone = PLAN_EFFECT_TONE[outcome.effect] ?? PLAN_EFFECT_TONE.no_change;

  return (
    <div className="row-divider flex items-start gap-4 px-4 py-3">
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14.5px] font-semibold">
          {outcome.name || outcome.email || "Name not available"}
        </span>
        <span className="block text-[13px] text-muted">
          {outcome.detail}
          {outcome.consequence ? (
            <>
              {" "}
              <span className="text-faint">{outcome.consequence}</span>
            </>
          ) : null}
        </span>
      </span>
      <span className={`shrink-0 rounded-pill px-2.5 py-1 text-[12.5px] font-semibold ${tone}`}>
        {label}
      </span>
    </div>
  );
}

/**
 * The note under the confirm button says what the button does NOT cover — the
 * rows already in the target state and the rows that were refused. Both are
 * counts an operator would otherwise discover only by reading the whole table.
 */
export function planNote(plan: BulkPlan, noun: [string, string] = ["person", "people"]): string {
  const parts: string[] = [];
  if (plan.summary.no_change > 0) parts.push(`No change for ${plan.summary.no_change}`);
  if (plan.summary.blocked > 0) parts.push(`${plan.summary.blocked} refused`);
  if (parts.length === 0) return `Every selected ${noun[0]} will change.`;
  return `${parts.join(" · ")} — Syndra leaves those as they are.`;
}

/** "Apply to 4 people" / "Nothing to apply". The button's own label states its scope. */
export function applyLabel(plan: BulkPlan, noun: [string, string]): string {
  const n = plan.summary?.apply;
  // A count that did not arrive is not a count. Rendering it produces "Apply to
  // undefined people" on the button that performs the change — which is both
  // the screen inventing a number and the screen lying about one, on the
  // control where it can least afford to. The action stays available, because a
  // backend that renamed a field should not block work; only the claim goes.
  if (typeof n !== "number") return "Apply the change";
  if (n === 0) return "Nothing to apply";
  return `Apply to ${n} ${n === 1 ? noun[0] : noun[1]}`;
}

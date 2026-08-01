"use client";

import type { BulkEffect, BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";

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

const EFFECT_STYLE: Record<BulkEffect, { label: string; className: string }> = {
  apply: { label: "Will change", className: "bg-accent-soft text-accent-text" },
  applied: { label: "Applied", className: "bg-accent-soft text-accent-text" },
  no_change: { label: "No change", className: "bg-tint-2 text-muted" },
  blocked: { label: "Refused", className: "bg-warn-soft text-warn-text" },
  failed: { label: "Failed", className: "bg-danger-soft text-danger-text" },
};

export function PlanReview({ plan }: { plan: BulkPlan | null }) {
  if (!plan) return null;

  return (
    <div className="px-6">
      <div className="max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong">
        {plan.outcomes.map((outcome) => (
          <PlanRow key={outcome.user_id} outcome={outcome} />
        ))}
      </div>
    </div>
  );
}

function PlanRow({ outcome }: { outcome: BulkOutcome }) {
  const style = EFFECT_STYLE[outcome.effect] ?? EFFECT_STYLE.no_change;

  return (
    <div className="row-divider flex items-start gap-4 px-4 py-3">
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14.5px] font-semibold">
          {outcome.name || outcome.email || "Unknown account"}
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
      <span className={`shrink-0 rounded-pill px-2.5 py-1 text-[12px] font-semibold ${style.className}`}>
        {style.label}
      </span>
    </div>
  );
}

/**
 * The note under the confirm button says what the button does NOT cover — the
 * rows already in the target state and the rows that were refused. Both are
 * counts an operator would otherwise discover only by reading the whole table.
 */
export function planNote(plan: BulkPlan): string {
  const parts: string[] = [];
  if (plan.summary.no_change > 0) parts.push(`${plan.summary.no_change} already in that state`);
  if (plan.summary.blocked > 0) parts.push(`${plan.summary.blocked} refused`);
  if (parts.length === 0) return "Every selected row will change.";
  return `${parts.join(" · ")} — untouched by this.`;
}

/** "Apply to 4 people" / "Nothing to apply". The button's own label states its scope. */
export function applyLabel(plan: BulkPlan, noun: [string, string]): string {
  const n = plan.summary.apply;
  if (n === 0) return "Nothing to apply";
  return `Apply to ${n} ${n === 1 ? noun[0] : noun[1]}`;
}

// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PlanReview } from "@/components/ui/PlanReview";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * A plan row names its person even when the backend surface sent only an id.
 * The mapping rehearsal did, and the row read "Name not available", which an
 * operator took for a broken group or person. The id now goes through the
 * resolver, and an id nothing can place is an honest "Unknown account".
 */
describe("PlanReview names", () => {
  const plan: BulkPlan = {
    op: "create_mapping",
    plan_id: "plan_1",
    applied: false,
    outcomes: [{ user_id: "366407994264519686", effect: "apply", detail: "Joins the Staff group." }],
    summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  } as BulkPlan;

  it("never says the name is not available", () => {
    render(<PlanReview plan={plan} />);
    expect(screen.queryByText(/name not available/i)).toBeNull();
    expect(screen.getByText("Unknown account")).toBeTruthy();
    expect(screen.getByText("366407994264519686")).toBeTruthy();
  });

  it("uses the name the plan carries when it has one", () => {
    render(
      <PlanReview
        plan={{ ...plan, outcomes: [{ ...plan.outcomes[0], name: "Aanya Rao" }] }}
      />,
    );
    expect(screen.getByText("Aanya Rao")).toBeTruthy();
  });
});

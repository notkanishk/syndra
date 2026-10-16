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

// A-06 §4: a plan row for a change that will be queued forecasts it, in the
// future tense, with the amber dot and no link. The link is for the result.
describe("PlanReview queued rows", () => {
  const plan = {
    op: "assign_role",
    plan_id: "plan_2",
    applied: false,
    outcomes: [
      { user_id: "u1", name: "Aanya Rao", effect: "queued", detail: "Supervisor always waits for you." },
      { user_id: "u2", name: "Ben Ito", effect: "apply", detail: "Gets Member." },
    ],
    summary: { total: 2, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
  } as BulkPlan;

  it("says it will wait, with an amber dot and no link", () => {
    const { container } = render(<PlanReview plan={plan} />);
    const word = screen.getByText("Will wait for you to send");
    expect(word.className).toContain("warn");
    expect(word.querySelector("span[aria-hidden]")?.className).toContain("bg-warn");
    expect(container.querySelector("a")).toBeNull();
    expect(screen.queryByText(/Waiting for you to send/)).toBeNull();
  });

  it("says it in the operator's words once the plan has run", () => {
    render(<PlanReview plan={{ ...plan, applied: true }} />);
    expect(screen.getByText("Waiting for you to send")).toBeTruthy();
    expect(screen.queryByText("Will wait for you to send")).toBeNull();
  });

  it("leaves other rows without a dot", () => {
    render(<PlanReview plan={plan} />);
    expect(screen.getByText("Will change").querySelector("span[aria-hidden]")).toBeNull();
  });
});

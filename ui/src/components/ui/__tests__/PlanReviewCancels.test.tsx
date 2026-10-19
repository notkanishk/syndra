// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PlanReview } from "@/components/ui/PlanReview";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

// A-04 §14.7: a row that would cancel a change still waiting says so in its
// consequence line, and under the plan one amber line repeats it with the
// quote, because the plan goes stale if anyone queues another change first.
// The backend's own sentence says "waiting to be sent", which reads as
// automatic; the plan words it the operator's way.

function plan(consequences: Array<string | undefined>): BulkPlan {
  return {
    op: "assign_bundle",
    plan_id: "plan_c",
    applied: false,
    outcomes: consequences.map((consequence, i) => ({
      user_id: `u${i}`,
      name: `Person ${i}`,
      effect: "apply",
      detail: "Gets Member.",
      consequence,
    })),
    summary: { total: consequences.length, apply: consequences.length, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  } as BulkPlan;
}

const one = "Cancels a change waiting to be sent: the removal of member queued 12 Oct (left the course).";

describe("PlanReview cancellations", () => {
  it("the row says it in the operator's words, never 'waiting to be sent'", () => {
    render(<PlanReview plan={plan([one])} />);
    expect(
      screen.getAllByText(/Cancels a change that is waiting for you to send: the removal of member queued 12 Oct/)[0],
    ).toBeTruthy();
    expect(screen.queryByText(/waiting to be sent/)).toBeNull();
  });

  it("one amber line under the plan repeats it with the quote and says the plan goes stale", () => {
    render(<PlanReview plan={plan([one])} />);
    const line = screen.getByRole("note");
    expect(line.textContent).toContain("Cancels a change that is waiting for you to send: the removal of member queued 12 Oct “left the course”.");
    expect(line.textContent).toContain("If anyone queues another first, this plan goes stale and nothing is cancelled unseen.");
    expect(line.className).toContain("text-warn-text");
  });

  it("counts across rows, and keeps what else the row said", () => {
    render(
      <PlanReview
        plan={plan([
          `Revoked once you send it. ${one}`,
          "Cancels 2 changes waiting to be sent: the grant of a queued 13 Oct; the removal of b queued 14 Oct (duplicate).",
          undefined,
        ])}
      />,
    );
    const line = screen.getByRole("note");
    expect(line.textContent).toContain("Cancels 3 changes that are waiting for you to send:");
    expect(line.textContent).toContain("“duplicate”");
    expect(screen.getByText(/Revoked once you send it\./)).toBeTruthy();
  });

  it("a plan that cancels nothing has no amber line", () => {
    render(<PlanReview plan={plan([undefined, "Revoked once you send it."])} />);
    expect(screen.queryByRole("note")).toBeNull();
  });
});

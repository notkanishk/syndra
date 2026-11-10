// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PlanReview } from "@/components/ui/PlanReview";
import type { AutoRetry } from "@/lib/outcome";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

// A queued row's word after the plan has run says who sends it, by the same
// reading the result sentence uses (goesItself): a departed member's lock in an
// entitlement check goes on its own, so its row must not say it waits for Send.
function applied(sendsOnItsOwn: boolean): BulkPlan {
  return {
    op: "converge_entitlements",
    applied: true,
    outcomes: [
      { user_id: "u1", name: "Rey Mysterio", effect: "queued", detail: "Switches the account off.", sends_on_its_own: sendsOnItsOwn },
    ],
    summary: { total: 1, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
  } as unknown as BulkPlan;
}

describe("a queued row's word says who sends it", () => {
  it.each<[string, boolean, AutoRetry, string]>([
    ["goes on its own with background sending on", true, true, "On its way"],
    ["waits when background sending is off", true, false, "Waiting for you to send"],
    ["promises nothing while that is unknown", true, null, "Not in TrueNAS yet"],
    ["an ordinary row waits for Send", false, true, "Waiting for you to send"],
  ])("%s", (_, itself, auto, word) => {
    render(<PlanReview plan={applied(itself)} auto={auto} system="TrueNAS" />);
    expect(screen.getByText(word)).toBeInTheDocument();
  });
});

// A give Zitadel didn't take: the server's own retry state decides, as it does
// for the result sentence (goesItself reads `retry.state`).
describe("a retrying row's word follows the server's retry state", () => {
  const retrying = (state: "retrying" | "halted"): BulkPlan =>
    ({
      op: "assign_role",
      applied: true,
      outcomes: [
        {
          user_id: "u1", name: "Rey Mysterio", effect: "queued", detail: "",
          wait_reason: "halted:retrying", retry: { state, try: 2, of: 6, next_try: "2099-11-05T08:35:00Z" },
        },
      ],
      summary: { total: 1, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
    }) as unknown as BulkPlan;

  it("still trying: on its way", () => {
    render(<PlanReview plan={retrying("retrying")} auto />);
    expect(screen.getByText("On its way")).toBeInTheDocument();
  });
  it("stopped trying: waits for you", () => {
    render(<PlanReview plan={retrying("halted")} auto />);
    expect(screen.getByText("Waiting for you to send")).toBeInTheDocument();
  });
});

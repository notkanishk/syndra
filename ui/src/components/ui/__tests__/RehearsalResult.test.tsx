import { describe, expect, it } from "vitest";

import { resultMessage, resultTone } from "@/components/ui/RehearsalDialog";
import type { BulkPlan, BulkSummary } from "@/lib/queries/useBulkGrants";

/**
 * The toast is where an apply's outcome actually reaches a person, and it used
 * to report `succeeded` alone. Rows MkAuth had written down but never got to
 * Zitadel were simply absent from the sentence, so a bulk removal whose drain
 * was refused announced "12 people updated" while every one of those roles was
 * still live.
 */
function plan(summary: Partial<BulkSummary>): BulkPlan {
  return {
    op: "remove_role",
    applied: true,
    outcomes: [],
    summary: {
      total: 0,
      apply: 0,
      no_change: 0,
      blocked: 0,
      failed: 0,
      succeeded: 0,
      queued: 0,
      ...summary,
    },
  };
}

const noun: [string, string] = ["person", "people"];

describe("the apply toast", () => {
  it("stays a plain confirmation when everything landed", () => {
    expect(resultMessage(plan({ succeeded: 12 }), noun)).toBe("12 people updated.");
    expect(resultTone(plan({ succeeded: 12 }))).toBe("success");
  });

  it("names rows that never reached Zitadel instead of counting them as updated", () => {
    const p = plan({ succeeded: 0, queued: 12 });
    expect(resultMessage(p, noun)).toBe("0 applied, 12 recorded but not yet in Zitadel.");
    // A warning, not an error: nothing was lost and the outbox will re-drive it.
    expect(resultTone(p)).toBe("warning");
  });

  it("keeps the three populations apart in one sentence", () => {
    const p = plan({ succeeded: 5, queued: 4, failed: 3 });
    expect(resultMessage(p, noun)).toBe(
      "5 applied, 4 recorded but not yet in Zitadel, 3 didn't go through.",
    );
    // A real failure outranks a queued row.
    expect(resultTone(p)).toBe("error");
  });

  it("says person, not people, for one", () => {
    expect(resultMessage(plan({ succeeded: 1 }), noun)).toBe("1 person updated.");
  });
});

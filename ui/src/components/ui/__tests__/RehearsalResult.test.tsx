import { describe, expect, it } from "vitest";

import { resultMessage, resultSentence, resultTone, resultWord } from "@/components/ui/RehearsalDialog";
import type { BulkPlan, BulkSummary } from "@/lib/queries/useBulkGrants";

/**
 * The toast is where an apply's outcome actually reaches a person, and it used
 * to report `succeeded` alone. Rows Syndra had written down but never got to
 * Zitadel were simply absent from the sentence, so a bulk removal whose drain
 * was refused announced "12 people updated" while every one of those roles was
 * still live.
 */
function plan(summary: Partial<BulkSummary>, op: BulkPlan["op"] = "remove_role"): BulkPlan {
  return {
    op,
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
    expect(resultMessage(plan({ succeeded: 12 }), noun)).toBe("Applied to 12 people.");
    expect(resultTone(plan({ succeeded: 12 }))).toBe("success");
  });

  it("names rows that never reached Zitadel instead of counting them as updated", () => {
    const p = plan({ succeeded: 0, queued: 12 });
    expect(resultMessage(p, noun)).toBe("Applied to 0 people. 12 people are waiting for you to send to Zitadel.");
    // A warning, not an error: nothing was lost and the outbox will re-drive it.
    expect(resultTone(p)).toBe("warning");
  });

  it("keeps the three populations apart in one sentence", () => {
    const p = plan({ succeeded: 5, queued: 4, failed: 3 });
    expect(resultMessage(p, noun)).toBe(
      "Applied to 5 people. 4 people are waiting for you to send to Zitadel. 3 people failed.",
    );
    // A real failure outranks a queued row.
    expect(resultTone(p)).toBe("error");
  });

  it("says person, not people, for one", () => {
    expect(resultMessage(plan({ succeeded: 1 }), noun)).toBe("Applied to 1 person.");
  });
});

/**
 * Applying something that reaches nobody.
 *
 * "Applied to 0 people." is true and reads as a failure. It is what an operator
 * got after publishing a bundle version nothing holds yet, or saving a mapping
 * before anybody has the role — both of which are the act succeeding. Found in
 * a browser on the dev deployment, on a result step no operator could reach
 * until publishing became appliable at all.
 */
describe("an apply that reached nobody", () => {
  it("reports the act rather than a headcount of nought", () => {
    const p = plan({});
    expect(resultMessage(p, noun)).toBe(
      "Done. Nobody's access changed, because nobody holds it yet.",
    );
    expect(resultTone(p)).toBe("success");
  });

  it("does not swallow the counts when people are waiting in the outbox", () => {
    // Same `succeeded: 0`, and emphatically not the same situation: twelve
    // people are recorded and not yet in Zitadel, which is the split the
    // queued population exists to keep visible.
    const p = plan({ total: 12, queued: 12 });
    expect(resultMessage(p, noun)).toBe(
      "Applied to 0 people. 12 people are waiting for you to send to Zitadel.",
    );
  });

  it("does not swallow the counts when people failed", () => {
    const p = plan({ total: 3, failed: 3 });
    expect(resultMessage(p, noun)).toBe("Applied to 0 people. 3 people failed.");
  });
});

/**
 * The result's word carries the act and the count (A-04 §11): "Given to 12
 * people", not an "Applied" pill. Verbs that have no settled past tense yet
 * (the mapping and bundle-version surfaces) return nothing, and the block keeps
 * its old words rather than inventing one.
 */
describe("the result's word", () => {
  it("names the act in the past tense, with the count", () => {
    expect(resultWord(plan({ succeeded: 12 }, "assign_role"), noun)).toBe("Given to 12 people");
    expect(resultWord(plan({ succeeded: 12 }, "assign_bundle"), noun)).toBe("Given to 12 people");
    expect(resultWord(plan({ succeeded: 3 }, "remove_role"), noun)).toBe("Revoked from 3 people");
    expect(resultWord(plan({ succeeded: 3 }, "remove_bundle"), noun)).toBe("Revoked from 3 people");
    expect(resultWord(plan({ succeeded: 1 }, "extend"), noun)).toBe("Extended for 1 person");
  });

  it("counts people waiting for you to send, because the word wears the amber dot then", () => {
    expect(resultWord(plan({ succeeded: 2, queued: 10 }, "assign_role"), noun)).toBe(
      "Given to 12 people",
    );
  });

  it("says nothing for a failure, a plan that reached nobody, or an act with no settled word", () => {
    expect(resultWord(plan({ succeeded: 4, failed: 1 }, "assign_role"), noun)).toBeUndefined();
    expect(resultWord(plan({}, "assign_role"), noun)).toBeUndefined();
    expect(resultWord(plan({ succeeded: 4 }, "edit_mapping"), noun)).toBeUndefined();
    expect(resultWord(plan({ succeeded: 4 }, "publish_bundle_version"), noun)).toBeUndefined();
  });

  it("does not repeat the word in the sentence beside it", () => {
    expect(resultSentence(plan({ succeeded: 12 }, "assign_role"), noun)).toBe("It has taken effect.");
    expect(resultSentence(plan({ succeeded: 0, queued: 12 }, "assign_role"), noun)).toBe(
      "12 people are waiting for you to send to Zitadel.",
    );
    expect(resultSentence(plan({ succeeded: 5, queued: 4 }, "assign_role"), noun)).toBe(
      "4 people are waiting for you to send to Zitadel.",
    );
  });
});

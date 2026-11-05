import { describe, expect, it } from "vitest";

import { queuedNote, queuedStep, resultMessage, resultSentence, resultTone, resultWord } from "@/components/ui/RehearsalDialog";
import { waitsForSend } from "@/lib/outcome";
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
    const p = plan({ succeeded: 0, queued: 12 }, "assign_role");
    expect(resultMessage(p, noun)).toBe("Applied to 0 people. 12 people are waiting for you to send to Zitadel.");
    // A warning, not an error: nothing was lost and the outbox will re-drive it.
    expect(resultTone(p)).toBe("warning");
  });

  it("keeps the three populations apart in one sentence", () => {
    const p = plan({ succeeded: 5, queued: 4, failed: 3 }, "assign_role");
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
    const p = plan({ total: 12, queued: 12 }, "assign_role");
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

// Re-audit R2: rows Syndra sends again on its own, and bulk removals, are not
// "waiting for you to send", and their result carries no Send link.
describe("queued rows say who sends them", () => {
  const NEXT = "2099-11-05T08:35:00Z";
  const retrying = (id: string) => ({
    user_id: id,
    name: id,
    email: `${id}@example.org`,
    effect: "queued" as const,
    detail: "Recorded in Syndra. Zitadel didn't answer, so Syndra sends it again on its own.",
    wait_reason: "halted:retrying",
    retry: { state: "retrying" as const, try: 2, of: 6, next_try: NEXT },
  });

  it("a give Zitadel didn't take goes on its own, with its next try", () => {
    const p = { ...plan({ succeeded: 1, queued: 2 }, "assign_role"), outcomes: [retrying("a"), retrying("b")] };
    expect(resultSentence(p, noun)).toBe("2 people are not in Zitadel yet; Syndra sends them on its own.");
    expect(queuedNote(p)).toBe("Recorded in Syndra. Zitadel didn't answer. Syndra sends them again at 14:05 (try 2 of 6).");
    const step = queuedStep(p);
    expect(step.sendsItself).toBe(true);
    expect(step.next).toMatch(/^Nothing for you to do\. Syndra sends them again at 14:05/);
    expect(waitsForSend({ kind: "queued", message: "x", ...step })).toBe(false);
  });

  it("a bulk removal goes on its own: no Send link", () => {
    const p = plan({ queued: 3 }, "remove_role");
    expect(resultSentence(p, noun)).toBe("3 people are not in Zitadel yet; Syndra sends them on its own.");
    expect(waitsForSend({ kind: "queued", message: "x", ...queuedStep(p) })).toBe(false);
  });

  it("a give whose role waits still waits for Send", () => {
    const waits = { ...retrying("c"), wait_reason: "role_waits", retry: undefined };
    const p = { ...plan({ queued: 2 }, "assign_role"), outcomes: [retrying("a"), waits] };
    expect(resultSentence(p, noun)).toBe(
      "1 person is waiting for you to send to Zitadel. 1 person is not in Zitadel yet; Syndra sends it on its own.",
    );
    expect(waitsForSend({ kind: "queued", message: "x", ...queuedStep(p) })).toBe(true);
  });
});

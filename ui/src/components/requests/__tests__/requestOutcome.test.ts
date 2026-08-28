import { describe, expect, it } from "vitest";

import { requestOutcome } from "@/components/requests/RequestsScreen";

describe("requestOutcome", () => {
  // The defect this function was extracted to close. Both views used to treat "settled and not
  // approved" as a denial, so the first withdrawal would have shown a member's own retraction
  // back to the operator as a refusal somebody made.
  it("never reads a withdrawal as a denial", () => {
    expect(requestOutcome("withdrawn").operator).toBe("Withdrawn");
    expect(requestOutcome("withdrawn").operator).not.toBe(
      requestOutcome("rejected").operator,
    );
    expect(requestOutcome("withdrawn").member).not.toBe(requestOutcome("rejected").member);
  });

  // A queue and somebody's own list are read by different people for different reasons: "Declined"
  // is what an operator recorded, "Not approved" is what happened to you.
  it("says the same fact in each register", () => {
    expect(requestOutcome("rejected")).toMatchObject({
      operator: "Declined",
      member: "Not approved",
    });
    expect(requestOutcome("pending")).toMatchObject({ operator: "Open", member: "Waiting" });
  });

  // A status this file has not been taught about is echoed back, not bucketed. The alternative
  // is a new backend state quietly rendering as an approval on a record of who may use a laser.
  it("passes an unknown status through rather than guessing", () => {
    const unknown = requestOutcome("escalated_to_committee");
    expect(unknown.operator).toBe("escalated_to_committee");
    expect(unknown.member).toBe("escalated_to_committee");
    expect(unknown.tone).not.toBe("accent");
  });
});

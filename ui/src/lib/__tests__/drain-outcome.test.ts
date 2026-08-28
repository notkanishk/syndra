import { describe, expect, it } from "vitest";

import { describeDrain } from "@/lib/drain-outcome";
import type { DrainResult } from "@/lib/queries/usePropagation";

function result(overrides: Partial<DrainResult> = {}): DrainResult {
  return { applied: 0, failed: 0, requeued: 0, errored: 0, halted: false, ...overrides };
}

describe("describeDrain", () => {
  it("celebrates only a clean pass", () => {
    const outcome = describeDrain(result({ applied: 12 }));
    expect(outcome.tone).toBe("success");
    expect(outcome.message).toBe("12 sent.");
  });

  // The regression this exists for: a pass that requeued eight changes used to
  // report "0 sent, 0 failed" and read as an idle queue.
  it("never reports a requeue as success", () => {
    const outcome = describeDrain(result({ requeued: 8 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toContain("8 waiting to be sent");
    expect(outcome.message).toContain("send again");
  });

  it("says a decided-but-unrecorded change needs another pass", () => {
    const outcome = describeDrain(result({ applied: 3, errored: 1 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toBe("3 sent · 1 not recorded — send again.");
    expect(outcome.detail).toMatch(/could not record/);
  });

  it("reports every non-zero count, in a fixed order", () => {
    const outcome = describeDrain(result({ applied: 1, failed: 2, requeued: 3, errored: 4 }));
    expect(outcome.message).toBe("1 sent · 2 failed · 3 waiting to be sent · 4 not recorded — send again.");
  });

  it("does not celebrate a terminal failure", () => {
    expect(describeDrain(result({ applied: 5, failed: 1 })).tone).toBe("warning");
  });

  it("distinguishes the three halt reasons", () => {
    expect(describeDrain(result({ halted: true, reason: "drain_in_progress" }))).toMatchObject({
      tone: "info",
    });
    const offline = describeDrain(result({ halted: true, reason: "zitadel_offline" }));
    expect(offline.tone).toBe("error");
    expect(offline.message).toMatch(/Nothing was sent/);
    const exhausted = describeDrain(
      result({ halted: true, reason: "max_retries_exceeded", applied: 4 }),
    );
    expect(exhausted.tone).toBe("error");
    expect(exhausted.message).toContain("4 sent");
    expect(exhausted.message).toMatch(/given up/);
  });

  // Given up is the one outcome sending again cannot fix, so the sentence has
  // to say what does — and never use the mechanism's words for it.
  it("tells the reader what to do about a change that was given up on", () => {
    const outcome = describeDrain(result({ applied: 2, failed: 1, exhausted: 1 }));
    expect(outcome.tone).toBe("error");
    expect(outcome.message).toBe("2 sent · 1 failed. Syndra has given up on one change.");
    expect(outcome.detail).toMatch(/from the person's page/);
    expect(outcome.detail).not.toMatch(/resum|drain|retries/i);
  });

  it("says so when there was nothing to do", () => {
    expect(describeDrain(result())).toMatchObject({ tone: "info", message: "Nothing was waiting." });
    expect(describeDrain(undefined).tone).toBe("info");
  });
});

/**
 * A send runs one pass per connected system and the passes fail
 * independently. The summary has to survive that: "halted" beside "9 sent" is
 * now an ordinary result, and an operator told "nothing was sent" when nine
 * changes landed has been told the wrong half.
 */
describe("a send with more than one connected system", () => {
  it("names which system was skipped, by its name, rather than calling the whole pass a failure", () => {
    const out = describeDrain(
      result({
        applied: 9,
        halted: true,
        reason: "zitadel_offline",
        halted_target: "zitadel",
        passes: [
          { target: "zitadel", applied: 0, failed: 0, requeued: 0, abandoned: 0, errored: 0, halted: true, reason: "zitadel_offline" },
          { target: "truenas", applied: 9, failed: 0, requeued: 0, abandoned: 0, errored: 0, halted: false },
        ],
      }),
    );

    expect(out.message).toContain("9 sent");
    expect(out.message).toContain("Zitadel was skipped");
    // Not an error: most of the work went through, and the part that did not is
    // named and still waiting.
    expect(out.tone).toBe("warning");
    expect(out.detail).toMatch(/waiting to be sent/i);
  });

  it("says nothing was sent when the only pass that could run did not", () => {
    const out = describeDrain(
      result({
        halted: true,
        reason: "target_unreachable",
        halted_target: "truenas",
        passes: [
          { target: "truenas", applied: 0, failed: 0, requeued: 0, abandoned: 0, errored: 0, halted: true, reason: "target_unreachable" },
        ],
      }),
    );

    expect(out.tone).toBe("error");
    expect(out.message).toMatch(/TrueNAS is not answering/);
    // The pre-flight is the point: an outage costs one probe, not a retry
    // budget per row.
    expect(out.detail).toMatch(/nothing counts against the retry limit/i);
  });
});

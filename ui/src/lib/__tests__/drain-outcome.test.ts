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
    expect(outcome.message).toBe("12 applied.");
  });

  // The regression this exists for: a pass that requeued eight writes used to
  // report "0 applied, 0 failed" and read as an idle queue.
  it("never reports a requeue as success", () => {
    const outcome = describeDrain(result({ requeued: 8 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toContain("8 still queued");
    expect(outcome.message).toContain("resume again");
  });

  it("says a decided-but-unrecorded write needs another pass", () => {
    const outcome = describeDrain(result({ applied: 3, errored: 1 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toBe("3 applied · 1 not recorded — resume again.");
    expect(outcome.detail).toMatch(/couldn't record/);
  });

  it("reports every non-zero count, in a fixed order", () => {
    const outcome = describeDrain(result({ applied: 1, failed: 2, requeued: 3, errored: 4 }));
    expect(outcome.message).toBe("1 applied · 2 failed · 3 still queued · 4 not recorded — resume again.");
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
    expect(exhausted.message).toContain("4 applied");
    expect(exhausted.message).toMatch(/out of retries/);
  });

  it("says so when there was nothing to do", () => {
    expect(describeDrain(result())).toMatchObject({ tone: "info", message: "Nothing was waiting." });
    expect(describeDrain(undefined).tone).toBe("info");
  });
});

/**
 * A drain runs one pass per registered target and the passes fail
 * independently. The summary has to survive that: "halted" beside "9 applied"
 * is now an ordinary result, and an operator told "nothing was sent" when nine
 * writes landed has been told the wrong half.
 */
describe("a drain with more than one target", () => {
  it("names which target did not run rather than calling the whole pass a failure", () => {
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

    expect(out.message).toContain("9 applied");
    expect(out.message).toContain("zitadel did not run");
    // Not an error: most of the work went through, and the part that did not is
    // named and still queued.
    expect(out.tone).toBe("warning");
    expect(out.detail).toMatch(/stay queued/i);
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
    expect(out.message).toMatch(/truenas is unreachable/i);
    // The pre-flight is the point: an outage costs one probe, not a retry
    // budget per row.
    expect(out.detail).toMatch(/no write spent a retry/i);
  });
});

import { describe, expect, it } from "vitest";

import { outcomeFromDrain, describeDrain } from "@/lib/drain-outcome";
import { waitsForSend } from "@/lib/outcome";
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
  // And, since the background drain retries them (owner, 5 Nov 2026), it
  // never tells the operator to send again: Syndra does, at each row's next try.
  it("never reports a requeue as success, nor as the operator's to resend", () => {
    const outcome = describeDrain(result({ requeued: 8 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toContain("8 to be sent again");
    expect(outcome.message).not.toMatch(/Send again|waiting for you/);
    expect(outcome.detail).toMatch(/Syndra sends them again on its own; each change below says when/);
    expect(outcome.sendsItself).toBe(true);
    expect(outcomeFromDrain(result({ requeued: 8 }))).toMatchObject({ kind: "queued", sendsItself: true });
  });

  it("says a decided-but-unrecorded change is settled at its next try", () => {
    const outcome = describeDrain(result({ applied: 3, errored: 1 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toBe("3 sent · 1 not recorded.");
    expect(outcome.detail).toMatch(/could not record/);
  });

  it("reports every non-zero count, in a fixed order", () => {
    const outcome = describeDrain(result({ applied: 1, failed: 2, requeued: 3, errored: 4 }));
    expect(outcome.message).toBe("1 sent · 2 failed · 3 to be sent again · 4 not recorded.");
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
    // No pass halts on a spent row any more (drain.go processRow), so the
    // reason has no words of its own: it reads as any early stop.
    const other = describeDrain(result({ halted: true, reason: "max_retries_exceeded", applied: 4 }));
    expect(other.message).toBe("4 sent. Stopped early.");
  });

  // Given up is the one outcome sending again cannot fix, so the sentence has
  // to say what does — and never use the mechanism's words for it. Some went,
  // so it is not "failed" (whose result would add "Nothing was changed").
  it("tells the reader what to do about a change that was given up on", () => {
    const outcome = describeDrain(result({ applied: 2, failed: 1, exhausted: 1 }));
    expect(outcome.tone).toBe("warning");
    expect(outcome.message).toBe("2 sent · 1 failed. Syndra stopped sending one change.");
    expect(outcome.next).toMatch(/Syndra stopped sending, on Pending changes/);
    expect(outcome.next).toMatch(/Drift/);
    expect(`${outcome.detail} ${outcome.next}`).not.toMatch(/resum|drain|retries/i);
  });

  // A refused row is terminal: Send won't move it, so the result never ends
  // on the "Waiting for you to send" link (re-audit (c)).
  it("a refused change ends on where it is, not on Send", () => {
    const outcome = outcomeFromDrain(result({ applied: 2, failed: 1 }));
    expect(outcome).toMatchObject({ kind: "queued", next: expect.stringMatching(/Syndra stopped sending/) });
    expect(waitsForSend(outcome)).toBe(false);
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
    expect(out.detail).toMatch(/waiting for you to send/i);
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

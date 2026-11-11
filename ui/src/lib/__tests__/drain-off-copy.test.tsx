// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { offlineNext } from "@/components/people/PersonSystems";
import { removalOutcome } from "@/components/people/RemovalDialog";
import { revokeOutcome } from "@/components/review/UnexplainedAccess";
import { queuedNote, queuedStep } from "@/components/ui/RehearsalDialog";
import { waitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { nextStepOf, SEND_IT, waitsForSend, type ActionOutcome } from "@/lib/outcome";
import { retireOutcome } from "@/lib/retry";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import { targetLabel } from "@/lib/nav";

/**
 * One-path 1.8o. With REVOCATION_DRAIN_ENABLED=false (`auto_retry: false` in the
 * indicators) nothing sends a revocation by itself, so no line may say it does:
 * each says it waits in Pending changes until someone presses Send, and ends on
 * that step. With the drain on, the lines are unchanged.
 */

const goesItself = (o: ActionOutcome) => o.sendsItself === true;
const sayingOnItsOwn = /on (its|their) own/i;
const text = (o: ActionOutcome) => `${o.message} ${o.detail ?? ""} ${nextStepOf(o)}`;

describe("a Drift revoke", () => {
  it("drain on: sends itself", () => {
    const o = revokeOutcome("halted:other", "Operator", undefined, true);
    expect(goesItself(o)).toBe(true);
    expect(o.message).toMatch(sayingOnItsOwn);
  });
  it("drain off: waits for Send and ends on Pending changes", () => {
    const o = revokeOutcome("halted:other", "Operator", undefined, false);
    expect(goesItself(o)).toBe(false);
    expect(text(o)).not.toMatch(sayingOnItsOwn);
    expect(o.message).toMatch(/waits in Pending changes until someone presses Send/);
    expect(waitsForSend(o)).toBe(true);
    expect(nextStepOf(o)).toBe(SEND_IT);
  });
});

describe("retiring a bundle or a rule", () => {
  it("drain on: a halted revocation goes on its own", () => {
    const o = retireOutcome(2, "halted:zitadel_offline", undefined, "", true);
    expect(goesItself(o)).toBe(true);
    expect(text(o)).toMatch(sayingOnItsOwn);
  });
  it("drain off: the halted revocations wait for Send", () => {
    const o = retireOutcome(2, "halted:zitadel_offline", undefined, "", false);
    expect(goesItself(o)).toBe(false);
    expect(text(o)).not.toMatch(sayingOnItsOwn);
    expect(o.detail).toMatch(/wait under Pending changes until you send them/);
    expect(nextStepOf(o)).toBe(SEND_IT);
  });
  it("drain off with the server's own waiting state keeps its words", () => {
    const o = retireOutcome(1, "halted:retrying", { state: "waiting", try: 6, of: 6 }, "", false);
    expect(goesItself(o)).toBe(false);
    expect(nextStepOf(o)).toMatch(/Pending changes/);
  });
});

describe("removing a direct grant", () => {
  const halted = { wait_reason: "halted:zitadel_offline" } as Parameters<typeof removalOutcome>[0];
  it("drain on: Revocations send on their own", () => {
    const o = removalOutcome(halted, "Operator", undefined, true);
    expect(goesItself(o)).toBe(true);
    expect(o.message).toMatch(/Revocations send on their own/);
  });
  it("drain off: the revocation waits in Pending changes", () => {
    const o = removalOutcome(halted, "Operator", undefined, false);
    expect(goesItself(o)).toBe(false);
    expect(o.message).not.toMatch(sayingOnItsOwn);
    expect(o.message).toMatch(/waits in Pending changes until someone presses Send/);
    expect(nextStepOf(o)).toBe(SEND_IT);
  });
});

describe("a bulk revocation's note", () => {
  const plan: BulkPlan = {
    op: "remove_role",
    applied: true,
    outcomes: [],
    summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
  };
  it("drain on: sends revocations on their own, no Send step", () => {
    expect(queuedNote(plan, "Zitadel", true)).toMatch(/sends revocations on their own/);
    expect(queuedStep(plan, true).sendsItself).toBe(true);
  });
  it("drain off: waits for Send", () => {
    const note = queuedNote(plan, "Zitadel", false);
    expect(note).not.toMatch(sayingOnItsOwn);
    expect(note).toMatch(/waiting for you to send/);
    expect(note).toMatch(/Pending changes/);
    expect(queuedStep(plan, false)).toEqual({});
  });
});

describe("a send that left changes unanswered", () => {
  const requeued = { applied: 0, failed: 0, requeued: 2, errored: 0, exhausted: 0 } as Parameters<typeof outcomeFromDrain>[0];
  it("drain on: sent again on its own", () => {
    const o = outcomeFromDrain(requeued, targetLabel, true);
    expect(goesItself(o)).toBe(true);
  });
  it("drain off: waits for Send", () => {
    const o = outcomeFromDrain(requeued, targetLabel, false);
    expect(goesItself(o)).toBe(false);
    expect(o.detail).toMatch(/doesn't send them again on its own/);
    expect(nextStepOf(o)).toBe(SEND_IT);
  });
});

describe("while Zitadel is down", () => {
  it("drain on: sent on its own once it answers", () => {
    expect(waitsBeforeLine("remove", true)).toMatch(/sends it on its own once Zitadel answers/);
  });
  it("drain off: waits in Pending changes", () => {
    const line = waitsBeforeLine("remove", false);
    expect(line).not.toMatch(sayingOnItsOwn);
    expect(line).toMatch(/waits in Pending changes until someone presses Send/);
  });
});

describe("a person's system that is offline (PersonStorage)", () => {
  it("never promises 'nothing for you to do' for changes made meanwhile", () => {
    expect(offlineNext("TrueNAS", true)).toMatch(/Send on Pending changes/);
    expect(offlineNext("TrueNAS", true)).toMatch(/revocation then sends on its own/);
  });
  it("drain off: every change waits for Send", () => {
    const line = offlineNext("TrueNAS", false);
    expect(line).not.toMatch(sayingOnItsOwn);
    expect(line).toMatch(/press Send there/);
  });
});

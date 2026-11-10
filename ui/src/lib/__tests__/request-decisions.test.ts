import { describe, expect, it } from "vitest";

import { formatShortDate } from "@/lib/format";
import { DECLINED, approvalOutcome } from "@/lib/request-decisions";

// One approval vocabulary for every place a request is decided (3.13): the
// seat's note per wait_reason, in route-requests' words.
describe("approvalOutcome", () => {
  it("nothing waiting: applied, sent and not yet seen", () => {
    expect(approvalOutcome(undefined, "Charlotte", "Trained")).toMatchObject({
      kind: "applied",
      detail: "Sent · not yet seen in Zitadel",
    });
  });

  // W1: since PR 108 a 503 or a lost answer is requeued, not resent inline,
  // and the background drain sends it again. Never "Sent", never "can use it
  // now": it says when it is sent again (IST) and which try, and needs no Send.
  it("Zitadel didn't take it: says it is sent again at its next try, never Sent", () => {
    const out = approvalOutcome("halted:retrying", "Charlotte", "Trained", {
      retry: { state: "retrying", try: 2, of: 6, next_try: "2099-11-05T08:35:00Z" },
    });
    expect(out).toMatchObject({ kind: "queued", sendsItself: true });
    expect(out.message).toBe("Approved. Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6).");
    expect(`${out.message} ${out.detail}`).toMatch(/sends it again at 14:05/);
    expect(`${out.message} ${out.detail}`).not.toMatch(/\bSent\b|can use it now/);
  });

  // C2: an approval's whole effect, in the planner's words: the sentences
  // the row said before the press (A-21 §4.4, one voice).
  it("names what automatic rules also gave with the approval", () => {
    const bench = "Also gives Bench in Workshop, by the automatic rule from Trained in Workshop.";
    const one = approvalOutcome(undefined, "Charlotte", "Trained", undefined, [bench]);
    expect(one.message).toBe(`Approved. Charlotte can use it now. ${bench}`);
    const laser = "Also gives Laser in Fabrication, by the automatic rule from Bench in Workshop.";
    const two = approvalOutcome("halted:retrying", "Charlotte", "Trained", undefined, [bench, laser]);
    expect(two.message).toMatch(new RegExp(`${laser.replace(/[.]/g, "\\.")}$`));
    expect(approvalOutcome(undefined, "Charlotte", "Trained").message).not.toMatch(/automatic rule/i);
  });

  it("a role that waits: queued, and names the role", () => {
    expect(approvalOutcome("role_waits", "Charlotte", "Supervisor")).toMatchObject({
      kind: "queued",
      detail: "Supervisor always waits for you",
    });
  });

  it("Zitadel offline: queued, in the degraded frame's words", () => {
    expect(approvalOutcome("halted:zitadel_offline", "Charlotte", "Trained")).toMatchObject({
      kind: "queued",
      detail: "Zitadel didn't answer when it was approved",
    });
  });

  it("another stop: queued, with that stop's words", () => {
    expect(approvalOutcome("halted:drain_in_progress", "Charlotte", "Trained").detail).toMatch(
      /^Another send was running at \d\d:\d\d$/,
    );
  });

  it("an extension says where the end moved to", () => {
    expect(approvalOutcome(undefined, "Charlotte", "Trained", { effect: "extended", until: "2026-11-03T12:00:00Z" })).toMatchObject({
      kind: "applied",
      message: `Approved. Charlotte keeps it until ${formatShortDate("2026-11-03T12:00:00Z")}.`,
    });
    expect(approvalOutcome(undefined, "Charlotte", "Trained", { effect: "extended" }).message).toBe(
      "Approved. Charlotte keeps it with no end date.",
    );
  });

  it("one that ended while it waited says it was given again", () => {
    expect(approvalOutcome(undefined, "Charlotte", "Trained", { effect: "given_again", until: "2026-11-03T12:00:00Z" }).message).toBe(
      `Approved. It had ended, so Charlotte has it again until ${formatShortDate("2026-11-03T12:00:00Z")}.`,
    );
  });

  it("a wait_reason still wins over the effect", () => {
    expect(approvalOutcome("role_waits", "Charlotte", "Supervisor", { effect: "extended", until: "2026-11-03T12:00:00Z" })).toMatchObject({
      kind: "queued",
    });
  });

  it("a decline never claims anything was sent", () => {
    expect(DECLINED).toMatchObject({ detail: "Nothing was sent" });
    expect(DECLINED.kind).not.toBe("applied");
  });
});

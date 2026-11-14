import { describe, expect, it } from "vitest";

import { standingWords } from "@/lib/account-standing";
import type { Standing } from "@/lib/queries/useTargets";
import { NOTHING_TO_DO } from "@/lib/outcome";

describe("standing, in words", () => {
  it("carries the add-on's note and next step as given", () => {
    const w = standingWords({ usable: false, waiting_on: "member", note: "No password set yet", next: "Set a storage password on your page" }, "TrueNAS", "member");
    expect(w).toMatchObject({ state: "Waiting on you", note: "No password set yet", next: "Set a storage password on your page" });
  });

  it("names who is waited on, by audience", () => {
    expect(standingWords({ usable: false, waiting_on: "member" }, "TrueNAS", "operator").state).toBe("Waiting on the member");
    expect(standingWords({ usable: false, waiting_on: "operator" }, "TrueNAS", "member").state).toBe("Waiting on makerspace staff");
    expect(standingWords({ usable: false, waiting_on: "target" }, "TrueNAS", "operator").state).toBe("Waiting on TrueNAS");
  });

  it("never leaves the reader without a next step", () => {
    for (const waiting_on of ["member", "operator", "target", "none"] as const) {
      for (const usable of [true, false]) {
        for (const audience of ["member", "operator"] as const) {
          expect(standingWords({ usable, waiting_on }, "TrueNAS", audience).next.length).toBeGreaterThan(0);
        }
      }
    }
    expect(standingWords({ usable: true, waiting_on: "none" }, "TrueNAS", "member").next).toBe(NOTHING_TO_DO);
    expect(standingWords({ usable: false, waiting_on: "target" }, "TrueNAS", "member").next).toContain(NOTHING_TO_DO);
  });
});

describe("an account that is switched off", () => {
  const off: Standing = { usable: false, waiting_on: "none", note: "The account is switched off." };

  it("is expected, in one statement that stays in Syndra, and does not promise 'yet'", () => {
    const w = standingWords(off, "TrueNAS", "operator");
    expect(w.state).toBe("Switched off");
    expect(w.state).not.toMatch(/yet/i);
    expect(w.next).toBe(
      `${NOTHING_TO_DO} Syndra keeps it off while the person is on hold, deactivated or has no role that reaches TrueNAS.`,
    );
    expect(w.next).not.toMatch(/on on|switch it back on|by hand/i);
  });

  it("does not repeat the add-on's own 'switched off' note beside the state", () => {
    expect(standingWords(off, "TrueNAS", "operator").note).toBeUndefined();
    expect(standingWords({ ...off, note: "the account is switched off" }, "TrueNAS", "member").note).toBeUndefined();
  });

  it("keeps a note that says more than the state", () => {
    expect(standingWords({ ...off, note: "Held since 12 Nov." }, "TrueNAS", "operator").note).toBe("Held since 12 Nov.");
  });

  it("tells a member to ask staff rather than wait", () => {
    expect(standingWords(off, "TrueNAS", "member").next).toBe(
      "It stays off while you are on hold or deactivated. Ask makerspace staff if you did not expect this.",
    );
  });

  it("keeps the add-on's own next step when it gave one", () => {
    expect(standingWords({ ...off, next: "Ask the front desk." }, "TrueNAS", "operator").next).toBe("Ask the front desk.");
  });

  it("leaves a different waiting state as it was", () => {
    expect(standingWords({ usable: false, waiting_on: "target" } as Standing, "TrueNAS", "operator").state).toBe("Waiting on TrueNAS");
  });
});

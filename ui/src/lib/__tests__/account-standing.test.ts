import { describe, expect, it } from "vitest";

import { standingWords } from "@/lib/account-standing";
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

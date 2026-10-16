import { describe, expect, it } from "vitest";

import { DECLINED, approvalOutcome } from "@/lib/request-decisions";

// One approval vocabulary for every place a request is decided (3.13): the
// seat's note per wait_reason, in route-requests' words.
describe("approvalOutcome", () => {
  it("nothing waiting: applied, sent and not yet seen", () => {
    expect(approvalOutcome(undefined, "Ada", "Trained")).toMatchObject({
      kind: "applied",
      detail: "Sent · not yet seen in Zitadel",
    });
  });

  it("a role that waits: queued, and names the role", () => {
    expect(approvalOutcome("role_waits", "Ada", "Supervisor")).toMatchObject({
      kind: "queued",
      detail: "Supervisor always waits for you",
    });
  });

  it("Zitadel offline: queued, in the degraded frame's words", () => {
    expect(approvalOutcome("halted:zitadel_offline", "Ada", "Trained")).toMatchObject({
      kind: "queued",
      detail: "Zitadel didn't answer when it was approved",
    });
  });

  it("another stop: queued, with that stop's words", () => {
    expect(approvalOutcome("halted:drain_in_progress", "Ada", "Trained").detail).toMatch(
      /^Another send was running at \d\d:\d\d$/,
    );
  });

  it("a decline never claims anything was sent", () => {
    expect(DECLINED).toMatchObject({ detail: "Nothing was sent" });
    expect(DECLINED.kind).not.toBe("applied");
  });
});

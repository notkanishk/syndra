import { describe, expect, it } from "vitest";

import { waitIsChosen, waitReasonLine } from "@/lib/wait-reason";

describe("why a change waits", () => {
  it("says each reason in the step-left vocabulary", () => {
    expect(waitReasonLine("role_waits", "Supervisor")).toBe("Supervisor always waits for you");
    expect(waitReasonLine("halted:zitadel_offline", "Supervisor")).toBe("Zitadel didn't answer");
    expect(waitReasonLine("halted:zitadel_offline", "x", "2026-10-16T10:42:00")).toMatch(/^Zitadel didn't answer at \d\d:\d\d$/);
    expect(waitReasonLine("halted:drain_in_progress", "x")).toBe("Another send was running");
    expect(waitReasonLine("halted:pass_error", "x")).toBe("The send stopped");
    expect(waitReasonLine("manual", "x")).toBe("Its bundle or rule was set to wait");
    expect(waitReasonLine(undefined, "x")).toBeUndefined();
  });

  it("tells a chosen wait from a send that stopped", () => {
    expect(waitIsChosen("role_waits")).toBe(true);
    expect(waitIsChosen("manual")).toBe(true);
    expect(waitIsChosen("halted:zitadel_offline")).toBe(false);
  });
});

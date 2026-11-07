import { describe, expect, it } from "vitest";

import { ApiError, WouldCancelQueued } from "@/lib/api-client";
import { isStale, planKey, seenQuery } from "@/lib/usePlanConsent";

describe("what a DELETE carries of the plan it was shown", () => {
  it("is the fingerprint and the cancel ids, on the query", () => {
    const q = new URLSearchParams(seenQuery({ fingerprint: "fp1", cancel_ids: ["a", "b"] }));
    expect(q.get("fingerprint")).toBe("fp1");
    expect(q.get("cancel_ids")).toBe("a,b");
  });
  it("names no cancel ids when it cancels nothing, and nothing without a plan", () => {
    expect(seenQuery({ fingerprint: "fp1", cancel_ids: [] })).toBe("fingerprint=fp1");
    expect(seenQuery(undefined)).toBe("");
  });
});

describe("the plan", () => {
  it("lives under the person, so every access change reads it again", () => {
    expect(planKey({ op: "remove", userId: "u1", grantId: "g1" })).toEqual(["users", "u1", "plan", "remove", "g1"]);
  });
  it("is stale on PLAN_STALE and on the second safety, and nothing else", () => {
    expect(isStale(new ApiError(409, { error: "PLAN_STALE" }))).toBe(true);
    expect(isStale(new WouldCancelQueued([], "POST", "/x"))).toBe(true);
    expect(isStale(new ApiError(409, { error: "ALREADY_HELD" }))).toBe(false);
  });
});

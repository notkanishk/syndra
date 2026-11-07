import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { ApiError, WouldCancelQueued } from "@/lib/api-client";
import { fetchPlan, isStale, planKey, seenQuery } from "@/lib/usePlanConsent";

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
  // Tasks 1.2a and 1.3: extend and adopt are planned like every other act.
  it("asks for an extend by role and days, and an adoption by its drift row", async () => {
    const asked: string[] = [];
    vi.stubGlobal("fetch", (url: string) => {
      asked.push(url);
      return Promise.resolve(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    });
    try {
      const client = new QueryClient();
      await fetchPlan(client, { op: "extend", userId: "u1", projectId: "pLaser", roleKey: "member", days: 90 });
      await fetchPlan(client, { op: "adopt", userId: "u1", driftId: "d1" });
    } finally {
      vi.unstubAllGlobals();
    }
    expect(asked).toEqual([
      "/api/proxy/users/u1/plan?op=extend&project_id=pLaser&role_key=member&duration_days=90",
      "/api/proxy/users/u1/plan?op=adopt&drift_id=d1",
    ]);
  });
  it("is stale on PLAN_STALE and on the second safety, and nothing else", () => {
    expect(isStale(new ApiError(409, { error: "PLAN_STALE" }))).toBe(true);
    expect(isStale(new WouldCancelQueued([], "POST", "/x"))).toBe(true);
    expect(isStale(new ApiError(409, { error: "ALREADY_HELD" }))).toBe(false);
  });
});

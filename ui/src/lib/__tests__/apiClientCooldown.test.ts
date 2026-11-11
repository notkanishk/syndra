// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, request } from "@/lib/api-client";
import { cooldownOf } from "@/lib/queries/fresh";
import { COOLDOWN_REFUSED, cooldownNext, outcomeFromError } from "@/lib/outcome";

/**
 * A read refused inside the add-on's cooldown is a 429 carrying its wait, three
 * ways: the envelope (dormant list and removal), the inventory's own result
 * body with no code, and a bare Retry-After header. All three are one refusal;
 * the rate limiter's 429 is another thing and stays one.
 */
const answer = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(body), { status, headers })));

const refused = (): Promise<ApiError> => request("/targets/truenas/inventory").then(() => Promise.reject(new Error("did not throw")), (e) => e);

afterEach(() => vi.unstubAllGlobals());

describe("a 429 with a wait", () => {
  it("takes the envelope's own code and seconds", async () => {
    answer(429, { error: "FRESH_READ_COOLDOWN", message: "That target was read moments ago", retry_after: 12 }, { "Retry-After": "12" });
    const e = await refused();
    expect([e.status, e.code, e.retryAfter]).toEqual([429, "FRESH_READ_COOLDOWN", 12]);
  });

  it("takes the inventory's result body, which carries the same code, as the same refusal", async () => {
    answer(429, { error: "FRESH_READ_COOLDOWN", target: "truenas", halted: true, reason: "read_stale", retry_after: 9, unmanaged: null }, { "Retry-After": "9" });
    const e = await refused();
    expect([e.code, e.retryAfter]).toEqual(["FRESH_READ_COOLDOWN", 9]);
  });

  it("falls back to the Retry-After header when the body names no seconds", async () => {
    answer(429, { error: "FRESH_READ_COOLDOWN" }, { "Retry-After": "7" });
    expect((await refused()).retryAfter).toBe(7);
  });

  it("is not a cooldown without the code, whatever wait it carries (a proxy's own 429)", async () => {
    answer(429, { retry_after: 9 }, { "Retry-After": "9" });
    const e = await refused();
    expect(e.code).not.toBe("FRESH_READ_COOLDOWN");
    expect(cooldownOf(e)).toBeUndefined();
    vi.stubGlobal("fetch", vi.fn(async () => new Response("<html>429</html>", { status: 429, headers: { "Retry-After": "9" } })));
    expect(cooldownOf(await refused())).toBeUndefined();
  });

  it("leaves the rate limiter's 429 and a 429 with no wait as they were", async () => {
    answer(429, { error: "RATE_LIMITED", message: "Slow down" }, { "Retry-After": "3" });
    const limited = await refused();
    expect([limited.code, limited.retryAfter]).toEqual(["RATE_LIMITED", undefined]);
    answer(429, "busy");
    expect((await refused()).code).toBe("UNKNOWN_ERROR");
  });
});

describe("as an outcome", () => {
  it("is a refusal that says nothing was changed and counts down to the same press", async () => {
    answer(429, { error: "FRESH_READ_COOLDOWN", message: "the server's own count that would not tick", retry_after: 12 });
    const o = outcomeFromError(await refused());
    expect(o).toMatchObject({ kind: "refused", message: COOLDOWN_REFUSED, retryAfter: 12, next: cooldownNext(12) });
    expect(o.message).not.toContain("12");
  });

  it("says what to do once the count is over, and every other error is untouched", async () => {
    expect(cooldownNext(0)).toBe("Press the same button once more.");
    answer(429, { error: "RATE_LIMITED", message: "Slow down" });
    expect(outcomeFromError(await refused())).toMatchObject({ message: "Slow down", retryAfter: undefined });
  });
});

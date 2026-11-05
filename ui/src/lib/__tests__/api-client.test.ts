import { describe, it, expect, afterEach, vi } from "vitest";

import { request, ApiError, LostReply } from "@/lib/api-client";

// Fetch stub returning a JSON Response with the given status.
function respondWith(status: number, body: unknown): typeof fetch {
  return vi.fn(
    async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  ) as unknown as typeof fetch;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("request preserveErrorBody", () => {
  it("returns the parsed body on non-2xx instead of throwing when the flag is set", async () => {
    global.fetch = respondWith(500, { error: "boom", detail: "x" });
    const body = await request<{ error: string }>("zitadel/health", { preserveErrorBody: true });
    expect(body.error).toBe("boom");
  });

  it("throws ApiError on non-2xx without the flag", async () => {
    global.fetch = respondWith(500, { error: "boom" });
    await expect(request("zitadel/health")).rejects.toBeInstanceOf(ApiError);
  });
});

describe("a reply that never arrives", () => {
  const down = () => vi.fn(async () => { throw new TypeError("Failed to fetch"); }) as unknown as typeof fetch;

  it("is a LostReply, still a TypeError, for every write", async () => {
    for (const method of ["POST", "PUT", "PATCH", "DELETE"]) {
      global.fetch = down();
      const e = await request("things", { method }).catch((x) => x);
      expect(e).toBeInstanceOf(LostReply);
      expect(e).toBeInstanceOf(TypeError);
      expect((e as LostReply).method).toBe(method);
    }
  });

  it("is the plain error for a read", async () => {
    global.fetch = down();
    const e = await request("things").catch((x) => x);
    expect(e).not.toBeInstanceOf(LostReply);
    expect(e).toBeInstanceOf(TypeError);
  });
});

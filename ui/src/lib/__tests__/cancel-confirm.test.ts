import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, request } from "@/lib/api-client";
import { setCancelConfirmer, type QueuedChange } from "@/lib/cancel-confirm";

// Never invisibly (owner, 15 Oct): a refusal to cancel unshown queued changes
// becomes one question, and the answer decides whether anything is sent again.

const queued: QueuedChange = {
  id: "0b3c7a52-6f0e-4b8e-9a51-2d9b1c3e4f50",
  op_type: "revoke",
  user_id: "u1",
  project_id: "pNas",
  role_keys: ["community"],
  source: "direct",
  initiated_by: "op-2",
  queued_at: "2026-10-15T09:00:00Z",
};

function respond(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const refusal = { error: "WOULD_CANCEL_QUEUED", message: "This would cancel 1 change", cancels: [queued] };

afterEach(() => vi.unstubAllGlobals());

describe("an action that would cancel queued changes", () => {
  it("asks, then sends again naming exactly what was shown", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond(409, refusal))
      .mockResolvedValueOnce(respond(202, { status: "pending" }));
    vi.stubGlobal("fetch", fetchMock);
    const asked = vi.fn().mockResolvedValue(true);
    const unregister = setCancelConfirmer(asked);

    await expect(request("/users/u1/grants", { method: "POST", body: { role_key: "community" } })).resolves.toEqual({ status: "pending" });
    expect(asked).toHaveBeenCalledWith([queued]);
    const retried = new Headers(fetchMock.mock.calls[1][1].headers);
    expect(retried.get("X-Syndra-Cancel-Ack")).toBe(queued.id);
    expect(fetchMock.mock.calls[1][1].body).toBe(JSON.stringify({ role_key: "community" }));
    unregister();
  });

  it("kept, nothing is sent again and the surface hears that nothing changed", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(respond(409, refusal));
    vi.stubGlobal("fetch", fetchMock);
    const unregister = setCancelConfirmer(() => Promise.resolve(false));

    const error = await request("/users/u1/grants", { method: "POST", body: {} }).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("KEPT_QUEUED");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    unregister();
  });

  it("with nobody to ask, the answer is no", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(respond(409, refusal)));
    const error = await request("/users/u1/grants", { method: "POST", body: {} }).catch((e) => e);
    expect((error as ApiError).code).toBe("KEPT_QUEUED");
  });
});

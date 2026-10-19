import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, WouldCancelQueued, request } from "@/lib/api-client";
import { armAck, takeAck, type QueuedChange } from "@/lib/cancel-confirm";

// Never invisibly (owner, 15 Oct): a refusal to cancel unshown queued changes
// reaches the dialog that made the write, and a retry names only what it showed.

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

afterEach(() => {
  vi.unstubAllGlobals();
  takeAck("POST", "/users/u1/grants");
});

describe("an action that would cancel queued changes", () => {
  it("throws the refusal to the caller, typed, with the list, and sends nothing again", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(respond(409, refusal));
    vi.stubGlobal("fetch", fetchMock);

    const error = await request("/users/u1/grants", { method: "POST", body: {} }).catch((e) => e);
    expect(error).toBeInstanceOf(WouldCancelQueued);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as WouldCancelQueued).cancels).toEqual([queued]);
    expect((error as WouldCancelQueued).method).toBe("POST");
    expect((error as WouldCancelQueued).path).toBe("/users/u1/grants");
    expect((error as ApiError).code).toBe("WOULD_CANCEL_QUEUED");
    // The words an unwired caller shows are Syndra's, not the backend's.
    expect((error as ApiError).message).toBe("This would cancel 1 change that is waiting for you to send. Nothing was changed");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("an armed acknowledgement rides exactly the next request to that method and path", async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(respond(202, { status: "pending" })));
    vi.stubGlobal("fetch", fetchMock);

    armAck("POST", "/users/u1/grants", [queued.id, "second"]);
    await request("/users/u1/grants", { method: "POST", body: { role_key: "community" } });
    expect(new Headers(fetchMock.mock.calls[0][1].headers).get("X-Syndra-Cancel-Ack")).toBe(`${queued.id},second`);
    expect(fetchMock.mock.calls[0][1].body).toBe(JSON.stringify({ role_key: "community" }));

    await request("/users/u1/grants", { method: "POST", body: {} });
    expect(new Headers(fetchMock.mock.calls[1][1].headers).has("X-Syndra-Cancel-Ack")).toBe(false);
  });

  it("another write made meanwhile never carries it", async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(respond(202, {})));
    vi.stubGlobal("fetch", fetchMock);

    armAck("POST", "/users/u1/grants", [queued.id]);
    await request("/users/u2/grants", { method: "POST", body: {} });
    expect(new Headers(fetchMock.mock.calls[0][1].headers).has("X-Syndra-Cancel-Ack")).toBe(false);
    expect(takeAck("POST", "/users/u1/grants")).toEqual([queued.id]);
  });

  it("a refusal with no list is an ordinary conflict", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(respond(409, { error: "WOULD_CANCEL_QUEUED", cancels: [] })));
    const error = await request("/users/u1/grants", { method: "POST", body: {} }).catch((e) => e);
    expect(error).not.toBeInstanceOf(WouldCancelQueued);
    expect(error).toBeInstanceOf(ApiError);
  });
});

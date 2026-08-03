import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { Mock } from "vitest";
import { NextRequest } from "next/server";

import { makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/session", () => ({ getSession: vi.fn() }));
import { getSession } from "@/lib/session";
import { GET, POST, PUT, DELETE } from "@/app/api/proxy/[...path]/route";

const MEMBER = { id: "member-1", role: "user", sessionType: "demo" as const };
const ADMIN = { id: "admin-1", role: "admin", sessionType: "demo" as const };

type Handler = (
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> },
) => Promise<Response>;

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
});
afterEach(() => {
  vi.restoreAllMocks();
});

function call(
  handler: Handler,
  method: string,
  path: string[],
  opts?: { body?: unknown },
): Promise<Response> {
  const url = `http://localhost/api/proxy/${path.join("/")}`;
  const init: RequestInit = { method };
  if (opts?.body !== undefined) {
    init.body = JSON.stringify(opts.body);
    init.headers = { "content-type": "application/json" };
  }
  const req = new NextRequest(url, init);
  return handler(req, { params: Promise.resolve({ path }) });
}

describe("proxy route admin/member boundary", () => {
  it("returns 401 when there is no session", async () => {
    (getSession as Mock).mockResolvedValue(null);
    const res = await call(GET, "GET", ["catalog"]);
    expect(res.status).toBe(401);
    expect(proxy.calls.length).toBe(0);
  });

  it("forwards a member's own grants", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const res = await call(GET, "GET", ["users", MEMBER.id, "grants"]);
    expect(res.status).toBe(200);
    expect(proxy.calls.length).toBe(1);
  });

  it("blocks a member from another user's grants without hitting the backend", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const res = await call(GET, "GET", ["users", "other-9", "grants"]);
    expect(res.status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  it("allows member GET of catalog, applications, requests", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    for (const p of ["catalog", "applications", "requests"]) {
      const res = await call(GET, "GET", [p]);
      expect(res.status).toBe(200);
    }
  });

  it("blocks member GET of bundles without hitting the backend", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const res = await call(GET, "GET", ["bundles"]);
    expect(res.status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  it("forces requester_id to the session id on a member's POST /requests", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("POST", /\/api\/v1\/requests(\?|$)/, () => ({ ok: true }));
    const res = await call(POST, "POST", ["requests"], {
      body: { requester_id: "spoofed", note: "x" },
    });
    expect(res.status).toBe(200);
    const forwarded = proxy.calls.find((c) => c.method === "POST");
    expect((forwarded?.body as { requester_id?: string })?.requester_id).toBe(MEMBER.id);
  });

  it("blocks member mutations outside the routes a member owns", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    expect((await call(POST, "POST", ["bundles"], { body: {} })).status).toBe(403);
    expect((await call(PUT, "PUT", ["users", "x", "grants"], { body: {} })).status).toBe(403);
    expect((await call(DELETE, "DELETE", ["bundles", "b1"])).status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  // Every route below is self-only in the backend as well. These cases exist because the proxy
  // is the OUTER lock, and a route the backend guards correctly is still unreachable if this
  // list has not been told about it — which is exactly how the vault and the withdraw route
  // shipped dead.

  it("lets a member withdraw their own request", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("POST", /\/api\/v1\/requests\/r1\/withdraw/, () => ({ message: "ok" }));
    const res = await call(POST, "POST", ["requests", "r1", "withdraw"]);
    expect(res.status).toBe(200);
    expect(proxy.calls.length).toBe(1);
  });

  // The route carries no body, and the proxy must not invent one. It used to stamp requester_id
  // onto every member write, which put an unknown field into a body the backend decodes strictly.
  it("forwards the withdraw without inventing a body", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("POST", /\/api\/v1\/requests\/r1\/withdraw/, () => ({ message: "ok" }));
    await call(POST, "POST", ["requests", "r1", "withdraw"]);
    // Assert the call happened first — `calls[0]?.body` on an empty array is undefined too, and
    // this test would otherwise pass for a route the proxy refused outright.
    expect(proxy.calls.length).toBe(1);
    expect(proxy.calls[0].body).toBeUndefined();
  });

  it("reaches the member's own shadow credential, in every method it needs", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const base = ["users", MEMBER.id, "shadow-credential"];
    proxy.register("GET", /shadow-credential\/status/, () => ({ has_credential: false }));
    proxy.register("GET", /shadow-credential\/audit/, () => []);
    proxy.register("PUT", /shadow-credential/, () => ({ message: "ok" }));
    proxy.register("DELETE", /shadow-credential/, () => ({ message: "ok" }));

    expect((await call(GET, "GET", [...base, "status"])).status).toBe(200);
    expect((await call(GET, "GET", [...base, "audit"])).status).toBe(200);
    expect((await call(PUT, "PUT", base, { body: { password: "Correct-horse1!" } })).status).toBe(200);
    expect((await call(DELETE, "DELETE", base)).status).toBe(200);
    expect(proxy.calls.length).toBe(4);
  });

  // decodeJSONStrict rejects unknown fields, so an injected requester_id here would 400 a
  // password the member typed correctly.
  it("forwards a password body untouched", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("PUT", /shadow-credential/, () => ({ message: "ok" }));
    await call(PUT, "PUT", ["users", MEMBER.id, "shadow-credential"], {
      body: { password: "Correct-horse1!" },
    });
    expect(proxy.calls[0]?.body).toEqual({ password: "Correct-horse1!" });
  });

  it("blocks a member from somebody else's shadow credential", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    expect((await call(GET, "GET", ["users", "other-9", "shadow-credential", "status"])).status).toBe(403);
    expect(
      (await call(PUT, "PUT", ["users", "other-9", "shadow-credential"], { body: { password: "x" } }))
        .status,
    ).toBe(403);
    expect((await call(DELETE, "DELETE", ["users", "other-9", "shadow-credential"])).status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  // The PUT/DELETE exception is the credential itself, not everything under /users/{self}.
  it("does not let the credential exception widen to the rest of a member's own subtree", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    expect((await call(DELETE, "DELETE", ["users", MEMBER.id, "grants", "g1"])).status).toBe(403);
    expect((await call(PUT, "PUT", ["users", MEMBER.id, "access"], { body: {} })).status).toBe(403);
    expect(
      (await call(DELETE, "DELETE", ["users", MEMBER.id, "shadow-credential", "audit"])).status,
    ).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  it("filters GET /requests to the member's own rows", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("GET", /\/api\/v1\/requests(\?|$)/, () => [
      { id: "r1", requester_id: MEMBER.id },
      { id: "r2", requester_id: "someone-else" },
    ]);
    const res = await call(GET, "GET", ["requests"]);
    expect(res.status).toBe(200);
    const data = (await res.json()) as Array<{ id: string }>;
    expect(data).toHaveLength(1);
    expect(data[0].id).toBe("r1");
  });

  it("returns 502 when the backend is unreachable", async () => {
    (getSession as Mock).mockResolvedValue(ADMIN);
    global.fetch = vi.fn().mockRejectedValue(new Error("down")) as unknown as typeof fetch;
    const res = await call(GET, "GET", ["catalog"]);
    expect(res.status).toBe(502);
  });
});

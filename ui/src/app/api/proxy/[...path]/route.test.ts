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

  it("blocks member mutations other than POST /requests", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    expect((await call(POST, "POST", ["bundles"], { body: {} })).status).toBe(403);
    expect((await call(PUT, "PUT", ["users", "x", "grants"], { body: {} })).status).toBe(403);
    expect((await call(DELETE, "DELETE", ["bundles", "b1"])).status).toBe(403);
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

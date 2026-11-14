import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { Mock } from "vitest";
import { NextRequest } from "next/server";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/session", () => ({ getSession: vi.fn() }));
import { getSession } from "@/lib/session";
import * as proxyRoute from "@/app/api/proxy/[...path]/route";
import { GET, POST, PUT, DELETE, PATCH } from "@/app/api/proxy/[...path]/route";

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
  opts?: { body?: unknown; headers?: Record<string, string> },
): Promise<Response> {
  const url = `http://localhost/api/proxy/${path.join("/")}`;
  const init: NonNullable<ConstructorParameters<typeof NextRequest>[1]> = { method };
  if (opts?.headers) init.headers = opts.headers;
  if (opts?.body !== undefined) {
    init.body = JSON.stringify(opts.body);
    init.headers = { "content-type": "application/json", ...opts.headers };
  }
  const req = new NextRequest(url, init);
  return handler(req, { params: Promise.resolve({ path }) });
}

function rawCall(handler: Handler, method: string, path: string[], body: string): Promise<Response> {
  const req = new NextRequest(`http://localhost/api/proxy/${path.join("/")}`, {
    method, body, headers: { "content-type": "application/json" },
  });
  return handler(req, { params: Promise.resolve({ path }) });
}

describe("proxy route admin/member boundary", () => {
  // Replacing the action signing key is admin-only (action-key-autorotation):
  // no member allowlist entry, and the admin path forwards the strict body as sent.
  it("refuses a member replacing the action signing key without hitting the backend", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const res = await call(POST, "POST", ["zitadel", "action-key", "rotate"], { body: { force: true } });
    expect(res.status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  it("passes an admin's key replacement through, body unchanged", async () => {
    (getSession as Mock).mockResolvedValue(ADMIN);
    const res = await call(POST, "POST", ["zitadel", "action-key", "rotate"], { body: { force: true } });
    expect(res.status).toBe(200);
    expect(proxy.calls).toHaveLength(1);
    expect(proxy.calls[0].url).toMatch(/\/api\/v1\/zitadel\/action-key\/rotate$/);
    expect(proxy.calls[0].body).toEqual({ force: true });
  });

  // The backend decodes one rule (RFC 7493 §2.3: a name twice is refused).
  // JSON.parse keeps the last of two, so a re-serialised body would reach it
  // with the duplicate already settled; the proxy forwards the bytes sent.
  it("forwards the body's bytes, so the backend sees a duplicate name", async () => {
    (getSession as Mock).mockResolvedValue(ADMIN);
    const raw = '{"force":false, "force":true}';
    await rawCall(POST, "POST", ["zitadel", "action-key", "rotate"], raw);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[1]?.body).toBe(raw);
  });

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

  it("allows member GET of catalog, applications, requests, terms", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    for (const p of ["catalog", "applications", "requests", "terms"]) {
      const res = await call(GET, "GET", [p]);
      expect(res.status).toBe(200);
    }
  });

  // The rail's own read: a member's systems, by name, and nothing under it.
  it("forwards a member's own systems, and nothing under the prefix", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    expect((await call(GET, "GET", ["me", "systems"])).status).toBe(200);
    expect(proxy.calls.length).toBe(1);
    expect((await call(GET, "GET", ["me", "systems", "truenas"])).status).toBe(403);
    expect(proxy.calls.length).toBe(1);
  });

  it("blocks member GET of bundles without hitting the backend", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const res = await call(GET, "GET", ["bundles"]);
    expect(res.status).toBe(403);
    expect(proxy.calls.length).toBe(0);
  });

  // The proxy's own member goes in beside the bytes sent, never into a re-serialised copy, so a
  // requester_id of the member's own (or any name twice) reaches the backend as a duplicate,
  // which its decoder refuses (RFC 7493 §2.3), instead of JSON.parse settling it here.
  it("adds requester_id to a member's POST /requests beside the bytes sent", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    proxy.register("POST", /\/api\/v1\/requests(\?|$)/, () => ({ ok: true }));
    const sent = (raw: string) => {
      vi.mocked(global.fetch).mockClear();
      return rawCall(POST, "POST", ["requests"], raw).then(() => vi.mocked(global.fetch).mock.calls[0]?.[1]?.body);
    };
    expect(await sent(' {"note":"x"}')).toBe(' {"requester_id":"member-1","note":"x"}');
    expect(await sent('{"requester_id":"spoofed","note":"x"}')).toBe('{"requester_id":"member-1","requester_id":"spoofed","note":"x"}');
    expect(await sent('{"note":"a","note":"b"}')).toBe('{"requester_id":"member-1","note":"a","note":"b"}');
    expect(await sent("{ }")).toBe('{"requester_id":"member-1" }');
    expect(await sent("")).toBe('{"requester_id":"member-1"}');
    expect(await sent("{not json")).toBe("{not json");
    expect(await sent('["x"]')).toBe('["x"]');
  });

  it("adds the demo admin as granted_by and reviewer_id the same way", async () => {
    (getSession as Mock).mockResolvedValue(ADMIN);
    await rawCall(POST, "POST", ["users", "u1", "grants"], '{"role_key":"r","role_key":"s"}');
    await rawCall(POST, "POST", ["requests", "r1", "decision"], '{"status":"approved"}');
    const bodies = vi.mocked(global.fetch).mock.calls.map((c) => c[1]?.body);
    expect(bodies).toEqual(['{"granted_by":"admin-1","role_key":"r","role_key":"s"}', '{"reviewer_id":"admin-1","status":"approved"}']);
  });

  // An optional-body route (mark-external, the mapping delete) reads "no body" as consent to
  // defaults; a body it cannot read must reach it, to be its 400, never arrive as nothing.
  it("forwards a body that is not a JSON object as sent", async () => {
    (getSession as Mock).mockResolvedValue(ADMIN);
    for (const raw of ["{not json", '["x"]', "42", '{"reason":'])
      await rawCall(POST, "POST", ["drift", "d1", "mark-external"], raw);
    await rawCall(DELETE, "DELETE", ["rules", "mapping", "m1"], "{");
    expect(vi.mocked(global.fetch).mock.calls.map((c) => c[1]?.body)).toEqual(["{not json", '["x"]', "42", '{"reason":', "{"]);
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
    const body = (await res.json()) as { error: string; message: string; details?: { request_id?: string } };
    expect(body.error).toBe("BACKEND_UNREACHABLE");
    expect(body.message).not.toContain("down");
    expect(body.details?.request_id).toMatch(/^[0-9A-F]{8}$/);
  });

  it("speaks the backend's envelope for its own refusals", async () => {
    (getSession as Mock).mockResolvedValue(null);
    const res = await call(GET, "GET", ["catalog"]);
    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "UNAUTHORIZED", message: "Your session ended before this ran." });
  });
});

/** Every upstream answer shape the proxy has to carry through under its real status. */
function upstream(body: BodyInit | null, init?: ResponseInit) {
  const fn = vi.fn(async () => new Response(body, init));
  global.fetch = fn as unknown as typeof fetch;
  return fn;
}

describe("proxy route contract", () => {
  beforeEach(() => (getSession as Mock).mockResolvedValue(ADMIN));

  it("forwards PATCH with its body", async () => {
    const fn = upstream(JSON.stringify({ ok: true }));
    const res = await call(PATCH, "PATCH", ["targets", "mappings", "m1"], { body: { role: "x" } });
    expect(res.status).toBe(200);
    const [, init] = fn.mock.calls[0] as unknown as [URL, RequestInit];
    expect(init.method).toBe("PATCH");
    expect(init.body).toBe(JSON.stringify({ role: "x" }));
  });

  // The mapping delete carries its plan_id in the body; dropping it made the backend refuse it.
  it("forwards a DELETE body", async () => {
    const fn = upstream(JSON.stringify({ ok: true }));
    await call(DELETE, "DELETE", ["targets", "mappings", "m1"], { body: { plan_id: "p1" } });
    const [, init] = fn.mock.calls[0] as unknown as [URL, RequestInit];
    expect(init.body).toBe(JSON.stringify({ plan_id: "p1" }));
  });

  it("passes a 204 through as a 204 with no body", async () => {
    upstream(null, { status: 204 });
    const res = await call(DELETE, "DELETE", ["bundles", "b1"]);
    expect(res.status).toBe(204);
    expect(await res.text()).toBe("");
  });

  it("keeps a non-JSON error's real status and Retry-After but never its text", async () => {
    upstream("<html>rpc error: code = Unavailable 192.0.2.7</html>", { status: 503, headers: { "Retry-After": "7" } });
    const res = await call(GET, "GET", ["catalog"]);
    expect(res.status).toBe(503);
    const raw = await res.text();
    expect(raw).not.toContain("rpc error");
    expect(raw).not.toContain("192.0.2.7");
    const body = JSON.parse(raw) as { error: string; message: string; details: { request_id: string } };
    expect(body.error).toBe("UPSTREAM_ERROR");
    expect(body.details.request_id).toBeTruthy();
    expect(res.headers.get("retry-after")).toBe("7");
  });

  it("marks answers private, no-store and bounds the upstream call", async () => {
    const fn = upstream(JSON.stringify([]));
    const res = await call(GET, "GET", ["catalog"]);
    expect(res.headers.get("cache-control")).toBe("private, no-store");
    const [, init] = fn.mock.calls[0] as unknown as [URL, RequestInit];
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it("refuses a body over 1 MB before reading it", async () => {
    const fn = upstream("{}");
    const res = await call(POST, "POST", ["bundles"], {
      body: {},
      headers: { "content-length": String(1024 * 1024 + 1) },
    });
    expect(res.status).toBe(413);
    expect(fn).not.toHaveBeenCalled();
  });

  // The member allowlist matches on the segments as given; `..%2F` decodes to a step out of them.
  it("refuses a segment that decodes to a separator or dot-segment, even for a member", async () => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    const fn = upstream("{}");
    for (const bad of ["..%2Fbundles", "a%5Cb", "%2e%2e", "..", "%E0%A4%A"]) {
      const res = await call(GET, "GET", ["users", MEMBER.id, "grants", bad]);
      expect(res.status, bad).toBe(400);
    }
    expect(fn).not.toHaveBeenCalled();
  });

  it("refuses a write from another origin, and allows one from this deployment's own", async () => {
    const fn = upstream(JSON.stringify({ ok: true }));
    const foreign = await call(POST, "POST", ["bundles"], {
      body: {},
      headers: { origin: "https://evil.example" },
    });
    expect(foreign.status).toBe(403);
    expect(fn).not.toHaveBeenCalled();

    const same = await call(POST, "POST", ["bundles"], {
      body: {},
      headers: { origin: "http://localhost" },
    });
    expect(same.status).toBe(200);
    // Behind the reverse proxy the deployment's origin is the forwarded one.
    const forwarded = await call(POST, "POST", ["bundles"], {
      body: {},
      headers: {
        origin: "https://syndra.example.org",
        "x-forwarded-host": "syndra.example.org",
        "x-forwarded-proto": "https",
      },
    });
    expect(forwarded.status).toBe(200);
  });
});

// The proxy is a hand-listed set of exports; a method the router registers that it does not
// export is a route the UI can never reach (PATCH shipped that way).
describe("the proxy exports every method the backend router registers", () => {
  const router = readFileSync(
    resolve(__dirname, "../../../../../../backend/internal/handlers/router.go"),
    "utf8",
  );
  const methods = Array.from(
    new Set(
      Array.from(router.matchAll(/"(GET|POST|PUT|PATCH|DELETE) \/api\/v1\//g), (m) => m[1]),
    ),
  );

  it("finds the methods it is supposed to be checking", () => {
    expect(methods.length).toBeGreaterThan(0);
  });

  it.each(methods)("exports %s", (method) => {
    expect(typeof (proxyRoute as Record<string, unknown>)[method]).toBe("function");
  });
});

/**
 * §17 — the allowlist is bound to the backend's router, not to memory.
 *
 * The member surface shipped correct on both sides and unreachable in the
 * middle: `withUserAuth` on every `/me/` route, self-scoped handlers, and an
 * allowlist that had never heard of the prefix. Both suites passed, because
 * each was written against its own idea of what the routes were.
 *
 * This reads the router itself. `/api/v1/me/` is the member prefix by
 * definition — the subject comes from the session and there is no id in the
 * path to substitute — so a route there that a member cannot reach is always a
 * bug, and a new one (§26's doors) fails here the day it is added rather than
 * the day somebody signs in as a member.
 */
describe("the proxy allowlist against the backend's own router", () => {
  const router = readFileSync(
    resolve(__dirname, "../../../../../../backend/internal/handlers/router.go"),
    "utf8",
  );
  const memberRoutes = Array.from(
    router.matchAll(/"(GET|POST|PUT|DELETE) \/api\/v1\/(me\/[^"]*)"/g),
    (m) => ({ method: m[1], path: m[2] }),
  );

  it("finds the member routes it is supposed to be checking", () => {
    // A regex that silently matched nothing would make every assertion below
    // vacuous, which is the failure mode of every source-reading guard.
    expect(memberRoutes.length).toBeGreaterThan(0);
  });

  it.each(memberRoutes)("lets a member reach $method /$path", async ({ method, path }) => {
    (getSession as Mock).mockResolvedValue(MEMBER);
    // Path parameters are named by the router; any concrete value stands in,
    // because the allowlist matches on shape and the backend resolves the
    // subject from the session rather than from the path.
    const segments = path.split("/").map((s) => (s.startsWith("{") ? "truenas" : s));
    const handler = { GET, POST, PUT, DELETE }[method as "GET"];
    const res = await call(handler, method, segments, method === "GET" ? undefined : { body: {} });
    expect(res.status).not.toBe(403);
  });
});

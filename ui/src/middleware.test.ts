import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { createHmac } from "node:crypto";
import { NextRequest } from "next/server";

import { middleware } from "@/middleware";
import { SESSION_COOKIE_NAME } from "@/lib/session";

// Mirrors ADMIN_ONLY_PATHS in middleware.ts (not exported). Iterated so every
// gated route is covered by a single assertion loop.
const ADMIN_ONLY_PATHS = [
  "/applications",
  "/audit",
  "/bundles",
  "/graph",
  "/policies",
  "/projects",
  "/users",
];

const TEST_SECRET = "test-session-secret";

// Cookie encoding must match readSession's decode: signed base64url JSON
// (`payload.hmac`, SC4).
function cookie(payload: Record<string, unknown>): string {
  const body = Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
  const sig = createHmac("sha256", TEST_SECRET).update(body).digest("base64url");
  return `${body}.${sig}`;
}

// An attacker-minted cookie: valid payload, no valid signature.
function forgedCookie(payload: Record<string, unknown>): string {
  const body = Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
  return `${body}.${"A".repeat(43)}`;
}

function req(path: string, cookieValue?: string): NextRequest {
  const headers = new Headers();
  if (cookieValue !== undefined) headers.set("cookie", `${SESSION_COOKIE_NAME}=${cookieValue}`);
  return new NextRequest(`http://localhost${path}`, { headers });
}

const future = () => Math.floor(Date.now() / 1000) + 3600;
const past = () => Math.floor(Date.now() / 1000) - 3600;

function locationPath(res: Response): string {
  return new URL(res.headers.get("location")!).pathname;
}

const savedDomain = process.env.ZITADEL_DOMAIN;
const savedSecret = process.env.SESSION_SECRET;

describe("middleware admin/member boundary", () => {
  beforeEach(() => {
    delete process.env.ZITADEL_DOMAIN;
    process.env.SESSION_SECRET = TEST_SECRET;
  });
  afterEach(() => {
    if (savedDomain === undefined) delete process.env.ZITADEL_DOMAIN;
    else process.env.ZITADEL_DOMAIN = savedDomain;
    if (savedSecret === undefined) delete process.env.SESSION_SECRET;
    else process.env.SESSION_SECRET = savedSecret;
  });

  it("redirects a member off every admin-only path to /", async () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "user", expiresAt: future() });
    for (const p of ADMIN_ONLY_PATHS) {
      const res = await middleware(req(p, c));
      expect(res.status).toBe(307);
      expect(locationPath(res)).toBe("/");
    }
  });

  it("rejects a forged admin cookie (bad signature) and redirects to /login — SC4", async () => {
    const c = forgedCookie({ type: "oidc", userId: "attacker", role: "admin", expiresAt: future() });
    const res = await middleware(req("/users", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
  });

  it("rejects an unsigned (pre-SC4 legacy) cookie and redirects to /login", async () => {
    const unsigned = Buffer.from(
      JSON.stringify({ type: "oidc", userId: "u1", role: "admin", expiresAt: future() }),
      "utf8",
    ).toString("base64url");
    const res = await middleware(req("/users", unsigned));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
  });

  it("clears a stale demo cookie and redirects to /login when ZITADEL_DOMAIN is set", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    const c = cookie({ type: "demo", userId: "dev_admin", role: "admin" });
    const res = await middleware(req("/users", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain(`${SESSION_COOKIE_NAME}=`);
    expect(setCookie).toMatch(/Max-Age=0/i);
  });

  it("redirects an expired OIDC session to /login", async () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "admin", expiresAt: past() });
    const res = await middleware(req("/", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
  });

  it("redirects an authenticated session away from /login to /", async () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "user", expiresAt: future() });
    const res = await middleware(req("/login", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/");
  });

  it("lets a valid admin through an admin-only path (no redirect)", async () => {
    const c = cookie({ type: "oidc", userId: "a1", role: "admin", expiresAt: future() });
    const res = await middleware(req("/users", c));
    expect(res.headers.get("location")).toBeNull();
    expect(res.status).toBe(200);
  });
});

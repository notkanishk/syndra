import { describe, it, expect, beforeEach, afterEach } from "vitest";
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

// Cookie encoding must match readSession's decode: base64url JSON.
function cookie(payload: Record<string, unknown>): string {
  return Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
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

describe("middleware admin/member boundary", () => {
  beforeEach(() => {
    delete process.env.ZITADEL_DOMAIN;
  });
  afterEach(() => {
    if (savedDomain === undefined) delete process.env.ZITADEL_DOMAIN;
    else process.env.ZITADEL_DOMAIN = savedDomain;
  });

  it("redirects a member off every admin-only path to /", () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "user", expiresAt: future() });
    for (const p of ADMIN_ONLY_PATHS) {
      const res = middleware(req(p, c));
      expect(res.status).toBe(307);
      expect(locationPath(res)).toBe("/");
    }
  });

  it("clears a stale demo cookie and redirects to /login when ZITADEL_DOMAIN is set", () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    const c = cookie({ type: "demo", userId: "dev_admin", role: "admin" });
    const res = middleware(req("/users", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain(`${SESSION_COOKIE_NAME}=`);
    expect(setCookie).toMatch(/Max-Age=0/i);
  });

  it("redirects an expired OIDC session to /login", () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "admin", expiresAt: past() });
    const res = middleware(req("/", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/login");
  });

  it("redirects an authenticated session away from /login to /", () => {
    const c = cookie({ type: "oidc", userId: "u1", role: "user", expiresAt: future() });
    const res = middleware(req("/login", c));
    expect(res.status).toBe(307);
    expect(locationPath(res)).toBe("/");
  });

  it("lets a valid admin through an admin-only path (no redirect)", () => {
    const c = cookie({ type: "oidc", userId: "a1", role: "admin", expiresAt: future() });
    const res = middleware(req("/users", c));
    expect(res.headers.get("location")).toBeNull();
    expect(res.status).toBe(200);
  });
});

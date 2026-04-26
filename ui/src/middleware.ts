import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { SESSION_COOKIE_NAME } from "@/lib/session";

const ADMIN_ONLY_PATHS = [
  "/applications",
  "/audit",
  "/bundles",
  "/graph",
  "/policies",
  "/projects",
  "/users",
];

type SessionState =
  | { kind: "valid"; userId: string; role: string }
  | { kind: "missing" }
  | { kind: "stale-demo" };

function readSession(request: NextRequest): SessionState {
  const raw = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) return { kind: "missing" };

  try {
    const normalized = raw.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
    const decoded = atob(padded);
    const parsed = JSON.parse(decoded) as {
      type?: string;
      userId?: string;
      role?: string;
      expiresAt?: number;
    };

    if (parsed.type === "oidc") {
      if (!parsed.userId || (parsed.role !== "admin" && parsed.role !== "user")) return { kind: "missing" };
      // Reject expired OIDC tokens — redirect to login for re-auth
      if (typeof parsed.expiresAt === "number" && Date.now() / 1000 > parsed.expiresAt) return { kind: "missing" };
      return { kind: "valid", userId: parsed.userId, role: parsed.role };
    }

    // Demo or legacy (no type field). Mirrors the defense-in-depth guard in
    // lib/session.ts: when ZITADEL_DOMAIN is configured, a demo cookie is
    // necessarily stale and must not pass middleware. Signal `stale-demo`
    // so the redirect handler can actively clear the cookie — otherwise the
    // page-level getSession() would also reject it and the user would be
    // stuck in a redirect loop seeing an empty page.
    if (process.env.ZITADEL_DOMAIN) return { kind: "stale-demo" };
    if (!parsed.userId || (parsed.role !== "admin" && parsed.role !== "user")) return { kind: "missing" };
    return { kind: "valid", userId: parsed.userId, role: parsed.role };
  } catch {
    return { kind: "missing" };
  }
}

function redirectTo(request: NextRequest, path: string, clearSession = false) {
  const url = request.nextUrl.clone();
  url.pathname = path;
  url.search = "";
  const response = NextResponse.redirect(url, { status: 307 });
  if (clearSession) {
    // Expire immediately — the user lands on /login able to re-auth instead
    // of looping with a stale demo cookie that middleware keeps rejecting.
    response.cookies.set(SESSION_COOKIE_NAME, "", { maxAge: 0, path: "/" });
  }
  return response;
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/auth/") ||
    pathname.includes(".")
  ) {
    return NextResponse.next();
  }

  const session = readSession(request);

  if (session.kind === "stale-demo") {
    // Always send to /login and clear the stale cookie so the next request
    // is a clean unauthenticated state.
    return redirectTo(request, "/login", true);
  }

  if (session.kind === "missing" && pathname !== "/login") {
    return redirectTo(request, "/login");
  }

  if (session.kind === "valid" && pathname === "/login") {
    return redirectTo(request, "/");
  }

  if (session.kind === "valid" && session.role === "user" && ADMIN_ONLY_PATHS.some((entry) => pathname.startsWith(entry))) {
    return redirectTo(request, "/");
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api).*)"],
};

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Must match SESSION_COOKIE_NAME in lib/session.ts. Declared locally because
// this file runs on the Edge runtime and importing lib/session would pull
// node:crypto (used for cookie signing, SC4) into the Edge bundle.
const SESSION_COOKIE_NAME = "mkauth_session";

/**
 * Members reach exactly two destinations. Everything else is not rendered and
 * not reachable for them — the backend 403s the underlying reads regardless,
 * and this keeps a hand-typed URL from landing on a page that will only fail.
 *
 * An allowlist rather than a denylist on purpose: a new operator route added
 * to the rail is protected by default, instead of being exposed until somebody
 * remembers to add it here.
 */
const MEMBER_ALLOWED_PATHS = ["/", "/requests", "/login"];

type SessionState =
  | { kind: "valid"; userId: string; role: string }
  | { kind: "missing" }
  | { kind: "stale-demo" };

function base64urlDecode(value: string): string {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
  return atob(padded);
}

// Mirrors the HMAC check in lib/session.ts (SC4) on the Edge runtime: the
// cookie is `<payload>.<signature>`; an unsigned or tampered cookie must not
// pass middleware. crypto.subtle.verify is constant-time.
async function verifySignature(body: string, sig: string): Promise<boolean> {
  const secret = process.env.SESSION_SECRET || process.env.MKAUTH_API_KEY || "";
  if (!secret) return false;
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["verify"],
  );
  const sigBytes = Uint8Array.from(base64urlDecode(sig), (c) => c.charCodeAt(0));
  return crypto.subtle.verify("HMAC", key, sigBytes, new TextEncoder().encode(body));
}

async function readSession(request: NextRequest): Promise<SessionState> {
  const raw = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) return { kind: "missing" };

  try {
    const [body, sig] = raw.split(".");
    if (!body || !sig || !(await verifySignature(body, sig))) return { kind: "missing" };
    const decoded = base64urlDecode(body);
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

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/auth/") ||
    pathname.includes(".")
  ) {
    return NextResponse.next();
  }

  const session = await readSession(request);

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

  if (
    session.kind === "valid" &&
    session.role === "user" &&
    !MEMBER_ALLOWED_PATHS.includes(pathname)
  ) {
    return redirectTo(request, "/");
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api).*)"],
};

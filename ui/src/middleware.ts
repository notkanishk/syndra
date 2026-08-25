import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { buildRedirectUrl } from "@/lib/request-url";
// The member allowlist is `lib/nav`'s, not a copy of it. This file used to
// declare its own, and the two drifted the day the storage row was added: the
// rail offered every member a destination middleware then redirected them off.
// A route a member may reach is navigation structure, and navigation structure
// lives in one file. Safe on the Edge runtime where lib/session is not —
// lib/nav imports nothing at all, so nothing follows it into the bundle.
import { memberMayVisit } from "@/lib/nav";

// Must match SESSION_COOKIE_NAME in lib/session.ts. Declared locally because
// this file runs on the Edge runtime and importing lib/session would pull
// node:crypto (used for cookie signing, SC4) into the Edge bundle.
const SESSION_COOKIE_NAME = "syndra_session";

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
  const secret = process.env.SESSION_SECRET || process.env.SYNDRA_API_KEY || "";
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

function redirectTo(request: NextRequest, path: string, clearSession = false, search = "") {
  // nextUrl carries the address this process was reached on, not the one the
  // browser used, so cloning it sent every unauthenticated request to the
  // container's own host:port — the redirect the user hits on literally every
  // click before signing in.
  // `search` is passed explicitly rather than embedded in `path`:
  // `buildRedirectUrl` clears the query on purpose, so a `?next=` written into
  // the path would be dropped without a word.
  const url = buildRedirectUrl(request, path, search);
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
    // Carry where they were, so signing in returns them to it. Sessions here
    // last weeks and are met on personal phones: the way this is encountered
    // is a member tapping a link to their storage days later, and landing on
    // the home page after signing in means finding that link again.
    //
    // The path only — never the origin. `nextPath` is re-validated before it
    // is used, but the value that reaches the cookie-issuing routes should not
    // carry a host in the first place.
    const next = `${pathname}${request.nextUrl.search}`;
    return redirectTo(request, "/login", false, `?next=${encodeURIComponent(next)}`);
  }

  if (session.kind === "valid" && pathname === "/login") {
    return redirectTo(request, "/");
  }

  // `/login` needs no seat on the allowlist: a valid session was already sent
  // away from it two guards above, and an absent one never reaches this check.
  if (session.kind === "valid" && session.role === "user" && !memberMayVisit(pathname)) {
    return redirectTo(request, "/");
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api).*)"],
};

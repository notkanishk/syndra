import { isForeignOrigin } from "@/lib/request-url";
import { getSession } from "@/lib/session";
import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL || "http://backend:8080";
const API_KEY = process.env.SYNDRA_API_KEY || "";

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

const MAX_BODY_BYTES = 1024 * 1024;
// Above the slowest call made through here (a propagation drain issues real Zitadel mutations),
// so it only ever ends a hung upstream, never a working one.
const UPSTREAM_TIMEOUT_MS = 120_000;
// Per-member data, and a session-bound response: never kept by a shared cache.
const NO_STORE = { "Cache-Control": "private, no-store" };

/**
 * The proxy's own refusals speak the backend's envelope (handlers/errors.go): a stable code in
 * `error`, a plain sentence in `message`. Prose in `error` is what made the UI say "gave no reason".
 */
function refuse(status: number, error: string, message: string, extra?: { details?: Record<string, string>; headers?: Record<string, string> }) {
  return NextResponse.json(
    { error, message, ...(extra?.details ? { details: extra.details } : {}) },
    { status, headers: { ...NO_STORE, ...extra?.headers } },
  );
}

/** A 5xx the proxy itself speaks: raw text goes to the log under a reference, never to the screen. */
function upstreamFailure(status: number, code: string, message: string, raw: string, headers?: Record<string, string>) {
  const ref = crypto.randomUUID().replace(/-/g, "").slice(0, 8).toUpperCase();
  console.error(`[proxy] upstream failure status=${status} ref=${ref} code=${code}: ${raw.slice(0, 500)}`);
  return refuse(status, code, message, { details: { request_id: ref }, headers });
}

/**
 * A segment that decodes to a separator or a dot-segment would let `..%2F` walk out of the
 * subtree the allowlist checked: the list matches on the segments as given, the backend resolves
 * the joined URL. A malformed escape is refused for the same reason.
 */
function hasUnsafeSegment(path: string[]) {
  return path.some((segment) => {
    try {
      const s = decodeURIComponent(segment);
      return s.includes("/") || s.includes("\\") || s === "." || s === "..";
    } catch {
      return true;
    }
  });
}

/**
 * `/users/{self}/…` subtrees a member may reach.
 *
 * Matched on the segment after the id, so `shadow-credential` covers the vault's `/status` and
 * `/audit` reads as well as the credential itself. Every one of those routes is self-only in the
 * backend too — this is the outer of two locks, not the only one.
 */
function isSelfScoped(path: string[], userId: string) {
  if (path[0] !== "users" || path[1] !== userId) return false;
  return path[2] === "access" || path[2] === "grants" || path[2] === "shadow-credential";
}

/**
 * The `/me/…` prefix: routes whose subject is the session rather than the path.
 *
 * There is no id to compare, which is the point of the prefix — the backend
 * resolves the subject from the authenticated session, so there is nothing here
 * for a caller to substitute. Every one is `withUserAuth` and self-scoped
 * there; this is the outer of two locks.
 *
 * Still an explicit list rather than a blanket `path[0] === "me"`. A prefix rule
 * would admit whatever gets added under it next, and the reason to enumerate
 * the reachable surface is that nothing should widen without somebody typing it.
 * `route.test.ts` reads the backend's router and fails when a `/me/` route is
 * missing here, so the list cannot fall behind either — which is the failure
 * that shipped: both sides of this boundary were correct and the middle had
 * never heard of the prefix.
 */
function isOwnMemberRoute(method: "GET" | "POST" | "PUT" | "DELETE", path: string[]) {
  if (path[0] !== "me") return false;
  const rest = path.slice(1);
  if (method === "GET") {
    // Their own name and team, their own view of every target, the systems
    // they meet (their rail's rows), and what changed in their own access lately.
    return rest.length === 1 && ["profile", "targets", "systems", "changes"].includes(rest[0]);
  }
  if (method === "POST") {
    // POST /me/first-sign-in (the sign-in route's own call, idempotent and self-scoped).
    if (rest.length === 1 && rest[0] === "first-sign-in") return true;
    // POST /me/targets/{target}/credential, and nothing else under the prefix.
    return rest.length === 3 && rest[0] === "targets" && rest[2] === "credential";
  }
  return false;
}

/** The member's own shadow credential, exactly — not its `/status` or `/audit` children. */
function isOwnShadowCredential(path: string[], userId: string) {
  return path.length === 3 && isSelfScoped(path, userId) && path[2] === "shadow-credential";
}

function isMemberAllowed(method: Method, path: string[], userId: string) {
  if (method === "GET") {
    if (path.length === 1 && (path[0] === "catalog" || path[0] === "applications" || path[0] === "requests" || path[0] === "terms")) {
      return true;
    }
    if (isOwnMemberRoute("GET", path)) return true;
    return isSelfScoped(path, userId);
  }

  if (method === "POST") {
    if (path.length === 1 && path[0] === "requests") return true;
    if (isOwnMemberRoute("POST", path)) return true;
    // Taking your own ask back. The backend scopes the UPDATE by the row's requester, so this
    // gate decides which route is reachable, never whose request is affected.
    return path.length === 3 && path[0] === "requests" && path[2] === "withdraw";
  }

  // PUT and DELETE are otherwise admin-only (PATCH wholly so). The single exception is the shadow
  // credential: it is the one object a member owns outright, and setting or clearing it changes
  // nobody's access — routing it through an operator would be asking somebody else to type your
  // password.
  if (method === "PUT" || method === "DELETE") {
    return isOwnShadowCredential(path, userId);
  }

  return false;
}

async function proxy(request: NextRequest, method: Method, path: string[]) {
  // The session cookie is SameSite=Lax, which a sibling subdomain still satisfies, so a write
  // must come from this deployment's own origin. A request with no Origin (curl, server-side) is
  // not a browser cross-site request and carries no ambient cookie to ride.
  if (method !== "GET" && isForeignOrigin(request)) {
    return refuse(403, "FORBIDDEN", "You don't have permission to do this.");
  }

  const session = await getSession();
  if (!session) {
    return refuse(401, "UNAUTHORIZED", "Your session ended before this ran.");
  }

  if (hasUnsafeSegment(path)) {
    return refuse(400, "BAD_REQUEST", "That request could not be read. Nothing was changed.");
  }

  if (session.role !== "admin" && !isMemberAllowed(method, path, session.id)) {
    return refuse(403, "FORBIDDEN", "You don't have permission to do this.");
  }

  if (method !== "GET" && Number(request.headers.get("content-length")) > MAX_BODY_BYTES) {
    return refuse(413, "REQUEST_TOO_LARGE", "That request is too large. Nothing was changed.");
  }

  const target = new URL(`${BACKEND_URL}/api/v1/${path.join("/")}`);
  target.search = request.nextUrl.search;

  if (session.sessionType === "oidc" && !session.accessToken) {
    return refuse(401, "UNAUTHORIZED", "Your session ended before this ran.");
  }

  const authToken = session.sessionType === "oidc" && session.accessToken
    ? session.accessToken
    : API_KEY;

  const headers: Record<string, string> = { Authorization: `Bearer ${authToken}` };
  // Demo mode has no token to carry a subject, so the request would otherwise
  // reach the backend anonymous and every `/me/*` route would resolve its actor
  // to "system" — no entitlement, no binding, no account. Sent only on the demo
  // path; an OIDC session carries its subject inside the token, where it
  // belongs, and the backend ignores this header entirely unless it is already
  // accepting the shared key.
  if (session.sessionType !== "oidc" && session.id) {
    headers["X-Syndra-Demo-Subject"] = session.id;
  }

  const init: RequestInit = {
    method,
    headers,
    cache: "no-store",
  };

  if (method !== "GET") {
    // Bodyless writes are legal (e.g. PUT /bundles/{id}/welcome — an
    // idempotent toggle with all state encoded in the path), and a DELETE may
    // carry one (the mapping delete's plan_id). Read the raw text and only
    // parse if non-empty; treat empty/malformed as "no body".
    const raw = await request.text();
    let body: Record<string, unknown> | null = null;
    if (raw.length > 0) {
      try {
        const parsed = JSON.parse(raw);
        body = parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : null;
      } catch {
        body = null;
      }
    }

    let payload: Record<string, unknown> | null = body;

    if (session.role !== "admin") {
      // Members can only create requests for themselves. Backend checks this
      // independently; the proxy enforces it here as defense-in-depth.
      //
      // Scoped to that ONE route, deliberately. This used to stamp requester_id onto every
      // member POST/PUT body, which was harmless only while filing a request was the sole thing
      // a member could write. It is not any more: the vault's `PUT {password}` would arrive
      // carrying an unknown field, and decodeJSONStrict rejects those — a 400 on a route that
      // was working, caused by the proxy adding something nobody asked for.
      if (method === "POST" && path.length === 1 && path[0] === "requests") {
        payload = { ...(body ?? {}), requester_id: session.id };
      }
    } else if (session.sessionType === "demo") {
      // Demo-mode admins authenticate via the shared API key, so the backend
      // can't derive an authenticated principal from the request. Inject the
      // demo session id as the audit actor for grant/request flows so audit
      // attribution stays meaningful in local dev. In OIDC mode the backend
      // resolves the actor from the JWT subject and ignores these fields.
      const isGrantWrite = method === "POST" && path[0] === "users" && path[2] === "grants";
      const isDecisionWrite = method === "POST" && path[0] === "requests" && path.length === 3 && path[2] === "decision";
      if (isGrantWrite) payload = { ...(body ?? {}), granted_by: session.id };
      else if (isDecisionWrite) payload = { ...(body ?? {}), reviewer_id: session.id };
    }

    if (payload !== null) {
      init.headers = {
        ...init.headers,
        "Content-Type": "application/json",
      };
      init.body = JSON.stringify(payload);
    }
    // payload === null → bodyless forward (no Content-Type, no body).
  }

  let res: Response;
  let text: string;
  try {
    res = await fetch(target, { ...init, signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS) });
    text = await res.text();
  } catch (e) {
    return upstreamFailure(502, "BACKEND_UNREACHABLE", "Syndra's server didn't answer.", String(e));
  }

  // The backend's own status and Retry-After are the answer; the proxy only decides the shape.
  const outHeaders: Record<string, string> = { ...NO_STORE };
  const retryAfter = res.headers.get("retry-after");
  if (retryAfter) outHeaders["Retry-After"] = retryAfter;

  // 204 (and any empty body) has nothing to parse, and a 204 must not carry a body at all.
  if (!text) return new Response(null, { status: res.status, headers: outHeaders });

  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    // A gateway's HTML error page or a plain-text body is the gateway's words, not the product's:
    // an error keeps its real status and Retry-After but speaks the envelope.
    if (!res.ok) return upstreamFailure(res.status, "UPSTREAM_ERROR", "Syndra's server could not answer that just now.", text, outHeaders);
    return new Response(text, {
      status: res.status,
      headers: { ...outHeaders, "Content-Type": "text/plain; charset=utf-8", "X-Content-Type-Options": "nosniff" },
    });
  }
  if (session.role !== "admin" && method === "GET" && path.length === 1 && path[0] === "requests" && Array.isArray(data)) {
    data = data.filter((entry) => entry?.requester_id === session.id);
  }
  return NextResponse.json(data, { status: res.status, headers: outHeaders });
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "GET", path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "POST", path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "PUT", path);
}

export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "PATCH", path);
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "DELETE", path);
}

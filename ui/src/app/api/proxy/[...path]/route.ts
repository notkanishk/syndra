import { getSession } from "@/lib/session";
import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL || "http://backend:8080";
const API_KEY = process.env.SYNDRA_API_KEY || "";

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

/** The member's own shadow credential, exactly — not its `/status` or `/audit` children. */
function isOwnShadowCredential(path: string[], userId: string) {
  return path.length === 3 && isSelfScoped(path, userId) && path[2] === "shadow-credential";
}

function isMemberAllowed(method: "GET" | "POST" | "PUT" | "DELETE", path: string[], userId: string) {
  if (method === "GET") {
    if (path.length === 1 && (path[0] === "catalog" || path[0] === "applications" || path[0] === "requests")) {
      return true;
    }
    return isSelfScoped(path, userId);
  }

  if (method === "POST") {
    if (path.length === 1 && path[0] === "requests") return true;
    // Taking your own ask back. The backend scopes the UPDATE by the row's requester, so this
    // gate decides which route is reachable, never whose request is affected.
    return path.length === 3 && path[0] === "requests" && path[2] === "withdraw";
  }

  // PUT and DELETE are otherwise admin-only. The single exception is the shadow credential: it is
  // the one object a member owns outright, and setting or clearing it changes nobody's access —
  // routing it through an operator would be asking somebody else to type your password.
  if (method === "PUT" || method === "DELETE") {
    return isOwnShadowCredential(path, userId);
  }

  return false;
}

async function proxy(request: NextRequest, method: "GET" | "POST" | "PUT" | "DELETE", path: string[]) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  if (session.role !== "admin" && !isMemberAllowed(method, path, session.id)) {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }

  const target = new URL(`${BACKEND_URL}/api/v1/${path.join("/")}`);
  target.search = request.nextUrl.search;

  if (session.sessionType === "oidc" && !session.accessToken) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const authToken = session.sessionType === "oidc" && session.accessToken
    ? session.accessToken
    : API_KEY;

  const init: RequestInit = {
    method,
    headers: {
      "Authorization": `Bearer ${authToken}`,
    },
    cache: "no-store",
  };

  if (method === "POST" || method === "PUT") {
    // Bodyless PUT/POST is legal (e.g. PUT /bundles/{id}/welcome — an
    // idempotent toggle with all state encoded in the path). Read the raw
    // text and only parse if non-empty; treat empty/malformed as "no body".
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

  try {
    const res = await fetch(target, init);
    const data = await res.json();
    if (session.role !== "admin" && method === "GET" && path.length === 1 && path[0] === "requests" && Array.isArray(data)) {
      return NextResponse.json(data.filter((entry) => entry?.requester_id === session.id), { status: res.status });
    }
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json({ error: "Backend unreachable" }, { status: 502 });
  }
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

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "DELETE", path);
}

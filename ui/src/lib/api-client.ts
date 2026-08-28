/**
 * Thin wrapper around fetch() targeting the Next.js API proxy under
 * /api/proxy/...  The proxy injects the authenticated session's bearer token,
 * so callers in the browser never see the upstream backend URL or the token.
 *
 * Non-2xx responses throw a typed ApiError so React Query's error state is
 * structured (status, code, message, optional details) — every call site can
 * branch on `error instanceof ApiError` and present the backend's machine
 * code (`error.code`) or human message (`error.message`).
 */

const API_BASE = "/api/proxy";

export interface ApiErrorBody {
  error?: string;
  message?: string;
  details?: Record<string, string>;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: Record<string, string>;

  constructor(status: number, body: ApiErrorBody | string) {
    const parsed: ApiErrorBody = typeof body === "string" ? { message: body } : body;
    super(parsed.message ?? "Syndra's server had a problem and gave no reason.");
    this.name = "ApiError";
    this.status = status;
    this.code = parsed.error ?? "UNKNOWN_ERROR";
    this.details = parsed.details;
  }
}

type RequestInitJSON = Omit<RequestInit, "body"> & {
  body?: unknown;
  /**
   * When true, a non-2xx response resolves with the parsed body (typed as T)
   * instead of throwing ApiError. Used by diagnostic probes (e.g. Zitadel
   * health) where the error envelope IS the payload the caller wants to read.
   */
  preserveErrorBody?: boolean;
};

/**
 * `request<T>(path, init?)` issues a JSON request to /api/proxy/<path> and
 * returns the parsed body on success. Use this everywhere instead of raw
 * fetch() — it is the single integration point with the backend.
 *
 * - The path argument is appended to /api/proxy (no leading slash required).
 * - When `body` is provided it's JSON-encoded and Content-Type is set.
 * - Non-2xx responses throw ApiError with a parsed body when available.
 * - 204 No Content responses resolve to `undefined as T`.
 */
export async function request<T = unknown>(path: string, init?: RequestInitJSON): Promise<T> {
  const url = path.startsWith("/") ? `${API_BASE}${path}` : `${API_BASE}/${path}`;

  // A mutation attempted with no network is refused here rather than sent.
  //
  // Refused, not failed, and the difference is what an operator does next: a
  // refusal means nothing was attempted and the state they are looking at is
  // the state that holds, where a failure leaves them wondering whether the
  // write half-landed. `fetch` on a dead network rejects with a TypeError
  // carrying no status, which reads as the second.
  //
  // Reads are left alone. A read that fails while offline fails harmlessly and
  // its list already has an error state; blocking it here would only replace
  // one honest failure with another.
  //
  // This is the whole of the offline write story. There is deliberately no
  // client-side queue: a queue in the browser is a second ledger nobody can
  // inspect, in a product whose argument is that Syndra decides and records.
  const method = (init?.method ?? "GET").toUpperCase();
  if (
    method !== "GET" &&
    typeof navigator !== "undefined" &&
    navigator.onLine === false
  ) {
    throw new ApiError(0, {
      error: "OFFLINE",
      message: "You're offline, so this wasn't sent",
    });
  }

  const headers = new Headers(init?.headers);
  let body: BodyInit | undefined;
  if (init?.body !== undefined) {
    body = JSON.stringify(init.body);
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  const res = await fetch(url, {
    ...init,
    headers,
    body,
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  let parsed: unknown = text;
  if (text.length > 0) {
    try {
      parsed = JSON.parse(text);
    } catch {
      // Leave as raw text; ApiError ctor handles either shape.
    }
  }

  if (!res.ok) {
    if (init?.preserveErrorBody) return parsed as T;
    // Session expired mid-SPA-session: /api/proxy is outside the middleware
    // matcher, so without this the user is stuck on a failing screen until
    // their next full navigation (SC9).
    //
    // A READ redirects to re-auth, carrying where they were so they come back
    // to it — a member who tapped a link to their storage wants storage, not
    // the landing.
    //
    // A MUTATION deliberately does not. Sessions here last weeks and are met
    // on personal phones, so the way this is actually encountered is: an
    // operator returns to a backgrounded tab, reads a plan, presses Apply, and
    // the session is gone. Navigating away at that moment destroys the dialog,
    // the plan they approved and any reason they had typed — and tells them
    // nothing about whether the write landed. The refusal is reported in place
    // instead, by the surface that ran it, which is the one thing this action
    // needs to say: nothing was changed, and the plan is still here.
    if (res.status === 401 && typeof window !== "undefined" && window.location.pathname !== "/login") {
      if (method === "GET") {
        const next = `${window.location.pathname}${window.location.search}`;
        window.location.assign(`/login?next=${encodeURIComponent(next)}`);
      } else {
        throw new ApiError(401, {
          error: "SESSION_ENDED",
          message: "Your session ended before this ran",
        });
      }
    }
    throw new ApiError(res.status, parsed as ApiErrorBody | string);
  }

  return parsed as T;
}

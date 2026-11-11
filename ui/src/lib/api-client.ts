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

import type { QueuedChange } from "@/lib/cancel-confirm";
import { SESSION_ENDED } from "@/lib/login-error";

/** The backend's code for a read refused inside the add-on's cooldown (429). */
export const FRESH_READ_COOLDOWN = "FRESH_READ_COOLDOWN";

const API_BASE = "/api/proxy";

export interface ApiErrorBody {
  error?: string;
  message?: string;
  details?: Record<string, string>;
  /** WOULD_CANCEL_QUEUED: what the action would have cancelled. */
  cancels?: QueuedChange[];
  /** FRESH_READ_COOLDOWN: whole seconds until the target may be read again. */
  retry_after?: number;
}

/** What a response with no envelope message says, by status, in the copy guide's words. */
function fallbackMessage(status: number): string {
  if (status === 401) return "Your session ended before this ran.";
  if (status === 403) return "You don't have permission to do this.";
  if (status === 404) return "That item no longer exists. Someone may have removed it.";
  if (status === 413) return "That request is too large. Nothing was changed.";
  if (status >= 500) return "Syndra's server had a problem.";
  return "Syndra's server had a problem and gave no reason.";
}

/**
 * A write whose reply never arrived: the request may have been sent and done.
 * Still a TypeError, so every reader of a lost reply (`lostReply`, the network
 * words) treats it as before; it only adds which method was lost.
 */
export class LostReply extends TypeError {
  constructor(
    cause: unknown,
    readonly method: string,
  ) {
    super(cause instanceof Error ? cause.message : "Failed to fetch");
    this.name = "LostReply";
  }
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: Record<string, string>;
  /** A 429's wait in whole seconds (its body's `retry_after`, else the Retry-After header). */
  readonly retryAfter?: number;
  /** The request's method, so a failed write is told apart from a failed read. */
  readonly method?: string;

  constructor(status: number, body: ApiErrorBody | string, method?: string) {
    // A bare string is a gateway's page or an upstream's text, never the product's words: at 5xx
    // it is not shown. The envelope's `message` is the only copy that reaches the screen.
    const parsed: ApiErrorBody =
      typeof body === "string" ? (status >= 500 ? {} : { message: body }) : body;
    super(parsed.message ?? fallbackMessage(status));
    this.name = "ApiError";
    this.status = status;
    this.code = parsed.error ?? "UNKNOWN_ERROR";
    this.details = parsed.details;
    this.retryAfter = parsed.retry_after;
    this.method = method;
  }
}

/**
 * The backend refused because this write would cancel changes still waiting for
 * you to send (409 WOULD_CANCEL_QUEUED). Nothing was changed. The dialog that
 * made the write shows `cancels` as a step inside itself, then sends the same
 * write again naming those ids in its `cancel_ids`. Any other caller reads it
 * as the refusal it is.
 */
export class WouldCancelQueued extends ApiError {
  readonly cancels: QueuedChange[];
  readonly method: string;
  readonly path: string;

  constructor(cancels: QueuedChange[], method: string, path: string) {
    const many = cancels.length !== 1;
    super(409, {
      error: "WOULD_CANCEL_QUEUED",
      message: `This would cancel ${cancels.length} ${many ? "changes that are" : "change that is"} waiting for you to send. Nothing was changed`,
    });
    this.name = "WouldCancelQueued";
    this.cancels = cancels;
    this.method = method;
    this.path = path;
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

  let res: Response;
  try {
    res = await fetch(url, {
      ...init,
      headers,
      body,
    });
  } catch (e) {
    // A read that fails fails harmlessly; a write may have been done.
    if (method !== "GET" && e instanceof TypeError) throw new LostReply(e, method);
    throw e;
  }

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
        window.location.assign(`/login?error=${SESSION_ENDED}&next=${encodeURIComponent(next)}`);
      } else {
        throw new ApiError(401, {
          error: "SESSION_ENDED",
          message: "Your session ended before this ran",
        });
      }
    }
    // The action would cancel changes still waiting for you to send. The caller
    // shows them; its retry names exactly the ids it showed (`cancel_ids`), and if
    // more was queued meanwhile the backend refuses again with the longer list.
    const refusal = parsed as ApiErrorBody;
    // A read refused inside the add-on's cooldown (429). The inventory's body is its read result,
    // not an envelope, so the code is the one the backend gives the envelope that does carry one.
    if (res.status === 429) {
      const header = Number.parseInt(res.headers.get("Retry-After") ?? "", 10);
      const wait = typeof refusal?.retry_after === "number" ? refusal.retry_after : header;
      if (wait > 0 && (refusal?.error ?? FRESH_READ_COOLDOWN) === FRESH_READ_COOLDOWN) {
        const { message } = typeof parsed === "object" && parsed !== null ? refusal : ({} as ApiErrorBody);
        throw new ApiError(429, { error: FRESH_READ_COOLDOWN, message, retry_after: wait }, method);
      }
    }
    if (res.status === 409 && refusal?.error === "WOULD_CANCEL_QUEUED" && refusal.cancels?.length) {
      throw new WouldCancelQueued(refusal.cancels, method, path);
    }
    throw new ApiError(res.status, parsed as ApiErrorBody | string, method);
  }

  return parsed as T;
}

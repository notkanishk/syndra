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
    super(parsed.message ?? `Request failed with status ${status}`);
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
    throw new ApiError(res.status, parsed as ApiErrorBody | string);
  }

  return parsed as T;
}

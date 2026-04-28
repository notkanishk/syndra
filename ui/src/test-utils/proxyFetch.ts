// @vitest-environment jsdom
import { vi } from "vitest";

/**
 * Routes a stubbed `fetch` against the Next.js proxy URLs the real app uses.
 * Tests register handlers like `register("GET /api/proxy/audit", () => […])`
 * and the helper picks the most specific match per request. Unknown paths
 * resolve to `200 []` so name resolution doesn't error if a name isn't
 * stubbed; the matchers below then assert that resolved names render.
 */
export type FetchHandler = (req: { url: string; init?: RequestInit; body?: unknown }) =>
  | unknown
  | Promise<unknown>;

interface RouteEntry {
  method: string;
  pattern: RegExp;
  handler: FetchHandler;
}

export function makeProxyFetch() {
  const routes: RouteEntry[] = [];
  const calls: Array<{ method: string; url: string; body?: unknown }> = [];

  function register(method: "GET" | "POST" | "PUT" | "DELETE", pattern: RegExp | string, handler: FetchHandler) {
    routes.push({
      method,
      pattern: typeof pattern === "string" ? new RegExp(`^${escapeRegex(pattern)}(?:\\?|$)`) : pattern,
      handler,
    });
  }

  const fetchImpl = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const method = (init?.method ?? "GET").toUpperCase();
    let body: unknown = undefined;
    if (init?.body && typeof init.body === "string") {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = init.body;
      }
    }
    calls.push({ method, url, body });

    for (const route of routes) {
      if (route.method !== method) continue;
      if (!route.pattern.test(url)) continue;
      const result = await route.handler({ url, init, body });
      return jsonResponse(result, 200);
    }
    return jsonResponse([], 200);
  });

  return { fetchImpl: fetchImpl as unknown as typeof fetch, calls, register };
}

function jsonResponse(value: unknown, status: number) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function escapeRegex(str: string) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Regex that matches a Zitadel-style UUID (8-4-4-4-12 hex). */
export const UUID_REGEX = /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/i;

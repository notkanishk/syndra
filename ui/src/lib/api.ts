import { getSession } from "@/lib/session";

// Server-side fetches go directly to the backend container
const SERVER_API = "http://backend:8080/api/v1";
// Client-side fetches go through our Next.js proxy route
const CLIENT_API = "/api/proxy";

const API_KEY = process.env.MKAUTH_API_KEY || "";
const OIDC_MODE = Boolean(process.env.ZITADEL_DOMAIN);

/**
 * Resolves the bearer token for a server-side backend request.
 *
 * - If a token is explicitly passed, use it (caller already has the session).
 * - If ZITADEL_DOMAIN is set (OIDC mode), read the session from cookies and
 *   return its access token. Throws if no valid token is found — falling back
 *   to the shared API key in OIDC mode would silently bypass the zero-trust
 *   guarantee, so we fail loudly instead.
 * - In demo mode (no ZITADEL_DOMAIN), fall back to the shared API key.
 */
async function resolveAuthToken(explicitToken?: string): Promise<string> {
  if (explicitToken) return explicitToken;

  if (OIDC_MODE) {
    const session = await getSession();
    const token = session?.accessToken;
    if (!token) {
      throw new Error(
        "SSR fetch: no user access token available in OIDC mode. " +
        "Ensure getSession() is called before fetching and ZITADEL_DOMAIN is correctly configured."
      );
    }
    return token;
  }

  return API_KEY;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function fetchServerJson(path: string, token?: string): Promise<any> {
  const authToken = await resolveAuthToken(token);
  const res = await fetch(`${SERVER_API}${path}`, {
    cache: "no-store",
    headers: { "Authorization": `Bearer ${authToken}` },
  });
  if (!res.ok) {
    throw new Error(`Failed to fetch ${path}: ${res.status}`);
  }
  return res.json();
}

export function getServerApiBase() {
  return SERVER_API;
}

export function getClientApiBase() {
  return CLIENT_API;
}

// Generic authenticated fetch for ad-hoc SSR calls in pages
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export async function fetchWithAuth(path: string, token?: string): Promise<any> {
  return fetchServerJson(path, token);
}

// --- Server-side fetchers (used in Server Components) ---
// The token parameter is optional: pass session.accessToken when you already
// have the session in scope to avoid a redundant cookie read, or omit it to
// let resolveAuthToken read it automatically.

export async function fetchBundles(token?: string) {
  return fetchServerJson("/bundles", token);
}

export async function fetchMappingRules(token?: string) {
  return fetchServerJson("/rules/mapping", token);
}

export async function fetchCatalog(token?: string) {
  return fetchServerJson("/catalog", token);
}

export async function fetchApplications(token?: string) {
  return fetchServerJson("/applications", token);
}

export async function fetchProjects(token?: string) {
  return fetchServerJson("/projects", token);
}

export async function fetchAudit(limit = 6, token?: string) {
  return fetchServerJson(`/audit?limit=${limit}`, token);
}

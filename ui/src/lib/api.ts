import { getSession } from "@/lib/session";
import type { ApplicationView } from "@/lib/types";

// Server-side fetches go directly to the backend container
const SERVER_API = `${process.env.BACKEND_URL || "http://backend:8080"}/api/v1`;

const API_KEY = process.env.SYNDRA_API_KEY || "";
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
        "SSR fetch: no access token for this person in OIDC mode. " +
        "Ensure getSession() is called before fetching and ZITADEL_DOMAIN is correctly configured."
      );
    }
    return token;
  }

  return API_KEY;
}

async function fetchServerJson<T = unknown>(path: string, token?: string): Promise<T> {
  const authToken = await resolveAuthToken(token);
  const res = await fetch(`${SERVER_API}${path}`, {
    cache: "no-store",
    headers: { "Authorization": `Bearer ${authToken}` },
  });
  if (!res.ok) {
    throw new Error(`Failed to fetch ${path}: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

// Generic authenticated fetch for ad-hoc SSR calls in pages
export async function fetchWithAuth<T = unknown>(path: string, token?: string): Promise<T> {
  return fetchServerJson<T>(path, token);
}

// --- Server-side fetchers (used in Server Components) ---
// The token parameter is optional: pass session.accessToken when you already
// have the session in scope to avoid a redundant cookie read, or omit it to
// let resolveAuthToken read it automatically.

export async function fetchApplications(token?: string): Promise<ApplicationView[]> {
  return fetchServerJson<ApplicationView[]>("/applications", token);
}

// SystemMode mirrors the backend SystemModeResponse. `directory` reports the
// active source ("zitadel" | "demo"); `degraded` is true iff the env requested
// live Zitadel but the directory fell back to demo (unexpected fallback).
export interface SystemMode {
  directory: "zitadel" | "demo";
  seed_active: boolean;
  zitadel_configured: boolean;
  degraded: boolean;
}

// fetchSystemMode reads /system/mode for chrome-level diagnostics. Returns
// null on any failure (auth, network, decoding) so the caller can render a
// silent steady-state instead of breaking the layout.
export async function fetchSystemMode(token?: string): Promise<SystemMode | null> {
  try {
    return await fetchServerJson<SystemMode>("/system/mode", token);
  } catch {
    return null;
  }
}

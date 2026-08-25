import { buildRedirectUrl } from "./request-url";

// ---------------------------------------------------------------------------
// PKCE cookie — bridges /auth/zitadel → /auth/callback
// ---------------------------------------------------------------------------

export const PKCE_COOKIE_NAME = "syndra_pkce";

interface PkceCookie {
  state: string;
  verifier: string;
  createdAt: number; // Unix seconds
  /**
   * Where to return the visitor once they are back, when they were somewhere
   * before the session lapsed.
   *
   * It rides in this cookie rather than in the OIDC `state` parameter: state
   * is a CSRF token whose only job is to be compared, and putting a
   * destination inside it would mean an attacker-composable value travelling
   * through the provider and back as part of the thing that proves the round
   * trip was ours. Here it stays on this origin, httpOnly, with the same
   * five-minute life as the verifier beside it — and it is validated again by
   * `safeReturnPath` before anybody is sent to it.
   *
   * Optional so a cookie written before this field existed still decodes: a
   * deployment mid-rollout should not fail a sign-in over a missing return
   * path.
   */
  next?: string;
}

export function encodePkce(payload: PkceCookie): string {
  return Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
}

export function decodePkce(value: string): PkceCookie | null {
  try {
    const decoded = Buffer.from(value, "base64url").toString("utf8");
    const parsed = JSON.parse(decoded) as Partial<PkceCookie>;
    if (
      typeof parsed.state !== "string" ||
      typeof parsed.verifier !== "string" ||
      typeof parsed.createdAt !== "number"
    ) {
      return null;
    }
    if (parsed.next !== undefined && typeof parsed.next !== "string") return null;
    return parsed as PkceCookie;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// PKCE crypto helpers
// ---------------------------------------------------------------------------

export async function generateCodeVerifier(): Promise<string> {
  const bytes = new Uint8Array(32);
  globalThis.crypto.getRandomValues(bytes);
  return bufferToBase64url(bytes);
}

export async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoded = new TextEncoder().encode(verifier);
  const digest = await globalThis.crypto.subtle.digest("SHA-256", encoded);
  return bufferToBase64url(new Uint8Array(digest));
}

export function generateState(): string {
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function bufferToBase64url(buffer: Uint8Array): string {
  let str = "";
  for (let i = 0; i < buffer.length; i += 1) {
    str += String.fromCharCode(buffer[i]);
  }
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, "");
}

// ---------------------------------------------------------------------------
// Redirect URI — must be byte-identical in both the initiation and callback
// routes (Zitadel validates this strictly).
// ---------------------------------------------------------------------------

export function buildCallbackUri(request: Request): string {
  return buildRedirectUrl(request, "/auth/callback").toString();
}

// ---------------------------------------------------------------------------
// Authorization URL
// ---------------------------------------------------------------------------

interface AuthorizationParams {
  domain: string;
  clientId: string;
  redirectUri: string;
  state: string;
  codeChallenge: string;
}

export function buildAuthorizationUrl(params: AuthorizationParams): string {
  const url = new URL(`https://${params.domain}/oauth/v2/authorize`);
  url.searchParams.set("client_id", params.clientId);
  url.searchParams.set("redirect_uri", params.redirectUri);
  url.searchParams.set("response_type", "code");
  url.searchParams.set("scope", "openid profile email");
  url.searchParams.set("state", params.state);
  url.searchParams.set("code_challenge", params.codeChallenge);
  url.searchParams.set("code_challenge_method", "S256");
  return url.toString();
}

// ---------------------------------------------------------------------------
// Token exchange
// ---------------------------------------------------------------------------

export interface TokenResponse {
  access_token: string;
  id_token?: string;
  token_type: string;
  expires_in: number;
}

interface ExchangeParams {
  domain: string;
  clientId: string;
  code: string;
  codeVerifier: string;
  redirectUri: string;
}

export async function exchangeCodeForToken(params: ExchangeParams): Promise<TokenResponse> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: params.clientId,
    code: params.code,
    code_verifier: params.codeVerifier,
    redirect_uri: params.redirectUri,
  });

  const res = await fetch(`https://${params.domain}/oauth/v2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`Token exchange failed (${res.status}): ${text}`);
  }

  return res.json() as Promise<TokenResponse>;
}

// ---------------------------------------------------------------------------
// JWT claim parsing (no signature verification — backend re-validates)
// ---------------------------------------------------------------------------

export function parseJwtClaims(jwt: string): Record<string, unknown> {
  const parts = jwt.split(".");
  if (parts.length !== 3) {
    throw new Error("Invalid JWT format");
  }
  // base64url → standard base64 → decode
  const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
  const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, "=");
  const decoded = atob(padded);
  return JSON.parse(decoded) as Record<string, unknown>;
}

// ---------------------------------------------------------------------------
// Session field extraction from Zitadel access token claims
// ---------------------------------------------------------------------------

interface ExtractedSessionFields {
  userId: string;
  name: string;
  email: string;
  role: "admin" | "user";
  expiresAt: number;
}

export function extractSessionFields(
  claims: Record<string, unknown>,
  adminRoleKey: string
): ExtractedSessionFields {
  const userId = typeof claims.sub === "string" ? claims.sub : "";
  // Never fall back to `sub`. A Zitadel access token carries profile claims
  // only when the instance is configured to inline userinfo; by default it
  // carries none, so a `?? userId` fallback here silently turned every
  // signed-in operator's display name into their opaque Zitadel id — in the
  // shell header and in the Home greeting, the two places a name is most
  // obviously a name. An empty string is the honest answer: the caller layers
  // /me/profile behind it, which does know.
  const name =
    typeof claims.name === "string" && claims.name.trim()
      ? claims.name
      : typeof claims.preferred_username === "string" && claims.preferred_username.trim()
        ? claims.preferred_username
        : "";
  const email = typeof claims.email === "string" ? claims.email : "";
  const expiresAt = typeof claims.exp === "number" ? claims.exp : 0;

  // Zitadel project roles claim: Record<roleKey, Record<orgId, orgName>>
  const projectRoles = claims["urn:zitadel:iam:org:project:roles"];
  let role: "admin" | "user" = "user";
  if (
    projectRoles !== null &&
    typeof projectRoles === "object" &&
    !Array.isArray(projectRoles) &&
    adminRoleKey in (projectRoles as Record<string, unknown>)
  ) {
    role = "admin";
  }

  return { userId, name, email, role, expiresAt };
}

// ---------------------------------------------------------------------------
// Avatar helper — derives initials from a display name
// ---------------------------------------------------------------------------

export function nameToAvatar(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}

// ---------------------------------------------------------------------------
// Profile metadata fetch — populates Title/Team for OIDC sessions
// ---------------------------------------------------------------------------

export interface ProfileMetadata {
  /**
   * The directory's own display name. Carried here because `/me/profile`
   * already returns it and the token often doesn't — dropping it on the floor
   * is what left the session holding a raw Zitadel id.
   */
  name: string;
  email: string;
  title: string;
  team: string;
  status: string;
}

/**
 * Fetches the authenticated user's profile from /api/v1/me/profile using the
 * freshly-issued access token. Returns empty metadata on any failure — the
 * OIDC callback must continue (the cookie is still valid, dashboard fields
 * just render blank). Backend is the canonical source; we never derive these
 * from token claims.
 */
export async function fetchProfileMetadata(
  accessToken: string,
  backendUrl: string,
): Promise<ProfileMetadata> {
  const empty: ProfileMetadata = { name: "", email: "", title: "", team: "", status: "active" };
  try {
    const res = await fetch(`${backendUrl}/api/v1/me/profile`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: "no-store",
    });
    if (!res.ok) return empty;
    const body = (await res.json()) as Partial<ProfileMetadata>;
    return {
      name: typeof body.name === "string" ? body.name : "",
      email: typeof body.email === "string" ? body.email : "",
      title: typeof body.title === "string" ? body.title : "",
      team: typeof body.team === "string" ? body.team : "",
      status: typeof body.status === "string" ? body.status : "active",
    };
  } catch {
    return empty;
  }
}

/**
 * Last resort before giving up on a name: "priya.sharma@example.org" reads
 * as "Priya Sharma". Still a person's name rather than an opaque id, which is
 * the whole point — a raw `sub` is never an acceptable display name.
 */
export function nameFromEmail(email: string): string {
  const local = email.split("@")[0] ?? "";
  if (!local.trim()) return "";
  return local
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join(" ");
}

/**
 * The display name for a session, in descending order of authority: the token
 * said so, the directory said so, or the email implies it. Never the id.
 * Returns "" when nothing knows — callers render an identity-less state rather
 * than leaking the subject id into the UI.
 */
export function resolveDisplayName(
  claimName: string,
  profileName: string,
  email: string,
): string {
  return claimName.trim() || profileName.trim() || nameFromEmail(email);
}

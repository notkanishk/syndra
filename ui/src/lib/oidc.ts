// ---------------------------------------------------------------------------
// PKCE cookie — bridges /auth/zitadel → /auth/callback
// ---------------------------------------------------------------------------

export const PKCE_COOKIE_NAME = "mkauth_pkce";

interface PkceCookie {
  state: string;
  verifier: string;
  createdAt: number; // Unix seconds
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
  const requestUrl = new URL(request.url);
  const forwardedHost = request.headers.get("x-forwarded-host");
  const forwardedProto = request.headers.get("x-forwarded-proto");

  const url = new URL(request.url);
  url.protocol = forwardedProto ? `${forwardedProto}:` : requestUrl.protocol;
  url.host = forwardedHost || request.headers.get("host") || requestUrl.host;
  url.pathname = "/auth/callback";
  url.search = "";
  url.hash = "";
  return url.toString();
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
  const name =
    typeof claims.name === "string"
      ? claims.name
      : typeof claims.preferred_username === "string"
        ? claims.preferred_username
        : userId;
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

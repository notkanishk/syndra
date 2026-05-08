import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import {
  buildCallbackUri,
  decodePkce,
  exchangeCodeForToken,
  extractSessionFields,
  fetchProfileMetadata,
  nameToAvatar,
  parseJwtClaims,
  PKCE_COOKIE_NAME,
} from "@/lib/oidc";
import { createOidcSessionValue, SESSION_COOKIE_NAME, type OidcSessionCookie } from "@/lib/session";

function redirectToLogin(request: Request, error: string): Response {
  const url = new URL(request.url);
  url.pathname = "/login";
  url.search = `?error=${encodeURIComponent(error)}`;
  return NextResponse.redirect(url, { status: 307 });
}

function buildRedirectUrl(request: Request, path: string): URL {
  const requestUrl = new URL(request.url);
  const forwardedHost = request.headers.get("x-forwarded-host");
  const forwardedProto = request.headers.get("x-forwarded-proto");

  const url = new URL(request.url);
  url.protocol = forwardedProto ? `${forwardedProto}:` : requestUrl.protocol;
  url.host = forwardedHost || request.headers.get("host") || requestUrl.host;
  url.pathname = path;
  url.search = "";
  return url;
}

export async function GET(request: Request): Promise<Response> {
  const domain = process.env.ZITADEL_DOMAIN;
  const clientId = process.env.ZITADEL_CLIENT_ID;
  const adminRoleKey = process.env.ZITADEL_ADMIN_ROLE_KEY ?? "admin";

  if (!domain || !clientId) {
    return redirectToLogin(request, "misconfigured");
  }

  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  const returnedState = url.searchParams.get("state");
  const errorParam = url.searchParams.get("error");

  // Zitadel can redirect back with ?error= on user denial or misconfiguration
  if (errorParam) {
    return redirectToLogin(request, errorParam);
  }

  if (!code || !returnedState) {
    return redirectToLogin(request, "missing_params");
  }

  const cookieStore = await cookies();
  const pkceCookieRaw = cookieStore.get(PKCE_COOKIE_NAME)?.value;

  if (!pkceCookieRaw) {
    return redirectToLogin(request, "pkce_missing");
  }

  const pkce = decodePkce(pkceCookieRaw);

  // Immediately invalidate the PKCE cookie regardless of outcome
  const requestUrl = new URL(request.url);
  const isSecure = request.headers.get("x-forwarded-proto") === "https" ||
    requestUrl.protocol === "https:";

  cookieStore.set(PKCE_COOKIE_NAME, "", {
    httpOnly: true,
    sameSite: "lax",
    secure: isSecure,
    path: "/auth/callback",
    maxAge: 0,
  });

  if (!pkce) {
    return redirectToLogin(request, "pkce_invalid");
  }

  // State mismatch → CSRF protection
  if (returnedState !== pkce.state) {
    return redirectToLogin(request, "state_mismatch");
  }

  // PKCE TTL check (defense-in-depth; cookie maxAge handles the common case)
  if (Math.floor(Date.now() / 1000) - pkce.createdAt > 300) {
    return redirectToLogin(request, "pkce_expired");
  }

  const redirectUri = buildCallbackUri(request);

  let tokenResponse;
  try {
    tokenResponse = await exchangeCodeForToken({
      domain,
      clientId,
      code,
      codeVerifier: pkce.verifier,
      redirectUri,
    });
  } catch {
    return redirectToLogin(request, "token_exchange_failed");
  }

  if (!tokenResponse.access_token) {
    return redirectToLogin(request, "no_access_token");
  }

  let claims: Record<string, unknown>;
  try {
    claims = parseJwtClaims(tokenResponse.access_token);
  } catch {
    return redirectToLogin(request, "invalid_token");
  }

  const fields = extractSessionFields(claims, adminRoleKey);

  if (!fields.userId) {
    return redirectToLogin(request, "invalid_claims");
  }

  const backendUrl = process.env.BACKEND_URL || "http://backend:8080";
  const profile = await fetchProfileMetadata(tokenResponse.access_token, backendUrl);

  const payload: OidcSessionCookie = {
    type: "oidc",
    accessToken: tokenResponse.access_token,
    userId: fields.userId,
    role: fields.role,
    name: fields.name || nameToAvatar(fields.userId), // fallback to derived string
    email: fields.email,
    title: profile.title,
    team: profile.team,
    location: profile.location,
    status: profile.status,
    expiresAt: fields.expiresAt,
  };

  const sessionValue = createOidcSessionValue(payload);
  const now = Math.floor(Date.now() / 1000);
  // Cap at 12 hours; subtract 30s skew buffer so cookie expires before JWT
  const maxAge = Math.min(Math.max(fields.expiresAt - now - 30, 0), 43200);

  cookieStore.set(SESSION_COOKIE_NAME, sessionValue, {
    httpOnly: true,
    sameSite: "lax",
    secure: isSecure,
    path: "/",
    maxAge,
  });

  return NextResponse.redirect(buildRedirectUrl(request, "/"), { status: 307 });
}

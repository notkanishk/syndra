import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import {
  buildCallbackUri,
  decodePkce,
  exchangeCodeForToken,
  extractSessionFields,
  fetchProfileMetadata,
  parseJwtClaims,
  resolveDisplayName,
  PKCE_COOKIE_NAME,
} from "@/lib/oidc";
import { buildRedirectUrl, isSecureRequest } from "@/lib/request-url";
import { safeReturnPath } from "@/lib/return-path";
import { createOidcSessionValue, SESSION_COOKIE_NAME, type OidcSessionCookie } from "@/lib/session";

function redirectToLogin(request: Request, error: string): Response {
  // Was built straight off request.url with no forwarded-host handling at all,
  // so every failure path bounced the browser to the container's own address.
  const url = buildRedirectUrl(request, "/login", `?error=${encodeURIComponent(error)}`);
  return NextResponse.redirect(url, { status: 307 });
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
  const isSecure = isSecureRequest(request);

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

  // Name resolution is layered because no single source is reliable: the
  // access token carries profile claims only if the Zitadel instance inlines
  // userinfo, and /me/profile is authoritative but can be unreachable. The one
  // thing that is never a name is `fields.userId`.
  const displayName = resolveDisplayName(fields.name, profile.name, fields.email || profile.email);

  const payload: OidcSessionCookie = {
    type: "oidc",
    accessToken: tokenResponse.access_token,
    userId: fields.userId,
    role: fields.role,
    name: displayName,
    email: fields.email || profile.email,
    title: profile.title,
    team: profile.team,
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

  // Back to where they were, if they were anywhere. Validated a third time,
  // here at the point of use: the value has been through a cookie and a
  // round trip since it was written, and this is the one place it becomes a
  // Location header.
  return NextResponse.redirect(buildRedirectUrl(request, safeReturnPath(pkce.next)), {
    status: 307,
  });
}

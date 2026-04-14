import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import {
  buildAuthorizationUrl,
  buildCallbackUri,
  encodePkce,
  generateCodeChallenge,
  generateCodeVerifier,
  generateState,
  PKCE_COOKIE_NAME,
} from "@/lib/oidc";

export async function GET(request: Request): Promise<Response> {
  const domain = process.env.ZITADEL_DOMAIN;
  const clientId = process.env.ZITADEL_CLIENT_ID;

  if (!domain || !clientId) {
    const url = new URL(request.url);
    url.pathname = "/login";
    url.search = "?error=misconfigured";
    return NextResponse.redirect(url, { status: 302 });
  }

  const state = generateState();
  const verifier = await generateCodeVerifier();
  const challenge = await generateCodeChallenge(verifier);
  const redirectUri = buildCallbackUri(request);

  const authUrl = buildAuthorizationUrl({
    domain,
    clientId,
    redirectUri,
    state,
    codeChallenge: challenge,
  });

  const requestUrl = new URL(request.url);
  const isSecure = request.headers.get("x-forwarded-proto") === "https" ||
    requestUrl.protocol === "https:";

  const cookieStore = await cookies();
  cookieStore.set(PKCE_COOKIE_NAME, encodePkce({ state, verifier, createdAt: Math.floor(Date.now() / 1000) }), {
    httpOnly: true,
    sameSite: "lax",
    secure: isSecure,
    path: "/auth/callback",
    maxAge: 300, // 5 minutes — longer than any realistic OIDC round-trip
  });

  return NextResponse.redirect(authUrl, { status: 302 });
}

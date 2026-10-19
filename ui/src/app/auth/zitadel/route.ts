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
import { buildRedirectUrl, isSecureRequest } from "@/lib/request-url";
import { safeReturnPath } from "@/lib/return-path";

export async function GET(request: Request): Promise<Response> {
  const domain = process.env.ZITADEL_DOMAIN;
  const clientId = process.env.ZITADEL_CLIENT_ID;

  if (!domain || !clientId) {
    const url = buildRedirectUrl(request, "/login", "?error=misconfigured");
    return NextResponse.redirect(url, { status: 302 });
  }

  // Validated on the way in as well as on the way out. It is stored on this
  // origin rather than sent to the provider, so the only thing that reaches
  // Zitadel is still the state token.
  const query = new URL(request.url).searchParams;
  const next = safeReturnPath(query.get("next"));
  const then = query.get("then") === "close" ? ("close" as const) : undefined;

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

  const isSecure = isSecureRequest(request);

  const cookieStore = await cookies();
  cookieStore.set(
    PKCE_COOKIE_NAME,
    encodePkce({ state, verifier, createdAt: Math.floor(Date.now() / 1000), next, then }),
    {
      httpOnly: true,
      sameSite: "lax",
      secure: isSecure,
      path: "/auth/callback",
      maxAge: 300, // 5 minutes — longer than any realistic OIDC round-trip
    },
  );

  return NextResponse.redirect(authUrl, { status: 302 });
}

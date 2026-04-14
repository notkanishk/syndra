import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { PKCE_COOKIE_NAME } from "@/lib/oidc";
import { SESSION_COOKIE_NAME } from "@/lib/session";

function buildRedirectUrl(request: Request, path: string) {
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

function redirectTo(request: Request, path: string) {
  return NextResponse.redirect(buildRedirectUrl(request, path), { status: 307 });
}

export async function POST(request: Request) {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
  cookieStore.delete({ name: PKCE_COOKIE_NAME, path: "/auth/callback" });
  return redirectTo(request, "/login");
}

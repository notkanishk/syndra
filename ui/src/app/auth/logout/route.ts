import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { PKCE_COOKIE_NAME } from "@/lib/oidc";
import { buildRedirectUrl } from "@/lib/request-url";
import { SESSION_COOKIE_NAME } from "@/lib/session";

function redirectTo(request: Request, path: string) {
  return NextResponse.redirect(buildRedirectUrl(request, path), { status: 307 });
}

export async function POST(request: Request) {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
  cookieStore.delete({ name: PKCE_COOKIE_NAME, path: "/auth/callback" });
  return redirectTo(request, "/login");
}

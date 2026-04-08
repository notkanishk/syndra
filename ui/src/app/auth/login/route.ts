import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { SESSION_COOKIE_NAME, createSessionValue } from "@/lib/session";

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
  const formData = await request.formData();
  const userId = formData.get("userId");
  if (typeof userId !== "string") {
    return redirectTo(request, "/login");
  }

  const sessionValue = createSessionValue(userId);
  if (!sessionValue) {
    return redirectTo(request, "/login");
  }

  const requestUrl = new URL(request.url);

  const cookieStore = await cookies();
  cookieStore.set(SESSION_COOKIE_NAME, sessionValue, {
    httpOnly: true,
    sameSite: "lax",
    secure: requestUrl.protocol === "https:",
    path: "/",
    maxAge: 60 * 60 * 12,
  });

  return redirectTo(request, "/");
}

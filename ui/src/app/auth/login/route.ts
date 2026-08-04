import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { buildRedirectUrl, isSecureRequest } from "@/lib/request-url";
import { SESSION_COOKIE_NAME, createSessionValue } from "@/lib/session";

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

  const cookieStore = await cookies();
  cookieStore.set(SESSION_COOKIE_NAME, sessionValue, {
    httpOnly: true,
    sameSite: "lax",
    secure: isSecureRequest(request),
    path: "/",
    maxAge: 60 * 60 * 12,
  });

  return redirectTo(request, "/");
}

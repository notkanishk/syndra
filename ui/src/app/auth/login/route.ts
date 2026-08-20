import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { buildRedirectUrl, isSecureRequest } from "@/lib/request-url";
import { safeReturnPath } from "@/lib/return-path";
import { SESSION_COOKIE_NAME, createSessionValue } from "@/lib/session";

function redirectTo(request: Request, path: string) {
  return NextResponse.redirect(buildRedirectUrl(request, path), { status: 307 });
}

export async function POST(request: Request) {
  const formData = await request.formData();
  const userId = formData.get("userId");
  // Where they were before the session lapsed, validated: a `next` is
  // attacker-composable, and unchecked it turns this deployment's own sign-in
  // into a redirect to somebody else's.
  const next = safeReturnPath(
    typeof formData.get("next") === "string" ? (formData.get("next") as string) : null,
  );
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

  return redirectTo(request, next);
}

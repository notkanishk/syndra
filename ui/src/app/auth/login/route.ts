import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { buildRedirectUrl, isForeignOrigin, isSecureRequest } from "@/lib/request-url";
import { safeReturnPath } from "@/lib/return-path";
import { doorCookie } from "@/lib/motion/door";
import { SESSION_COOKIE_NAME, createSessionValue, signedInNow } from "@/lib/session";

function redirectTo(request: Request, path: string) {
  return NextResponse.redirect(buildRedirectUrl(request, path), { status: 307 });
}

export async function POST(request: Request) {
  // A cross-site form must not be able to sign the visitor in as somebody else.
  if (isForeignOrigin(request)) {
    return new NextResponse(null, { status: 403 });
  }
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

  const signedInAt = signedInNow();
  const sessionValue = createSessionValue(userId, signedInAt);
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
  // The door is decided before the first paint (A-24 §3.2): the first page's server render reads this.
  cookieStore.set({ ...doorCookie(signedInAt), secure: isSecureRequest(request) });

  return redirectTo(request, next);
}

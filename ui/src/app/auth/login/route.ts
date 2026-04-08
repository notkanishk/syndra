import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { SESSION_COOKIE_NAME, createSessionValue } from "@/lib/session";

function redirectTo(path: string) {
  return NextResponse.redirect(path, { status: 307 });
}

export async function POST(request: Request) {
  const formData = await request.formData();
  const userId = formData.get("userId");
  if (typeof userId !== "string") {
    return redirectTo("/login");
  }

  const sessionValue = createSessionValue(userId);
  if (!sessionValue) {
    return redirectTo("/login");
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

  return redirectTo("/");
}

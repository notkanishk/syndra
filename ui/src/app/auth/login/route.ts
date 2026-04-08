import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { SESSION_COOKIE_NAME, createSessionValue } from "@/lib/session";

export async function POST(request: Request) {
  const formData = await request.formData();
  const userId = formData.get("userId");
  if (typeof userId !== "string") {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const sessionValue = createSessionValue(userId);
  if (!sessionValue) {
    return NextResponse.redirect(new URL("/login", request.url));
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

  return NextResponse.redirect(new URL("/", request.url));
}

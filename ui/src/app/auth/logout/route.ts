import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { SESSION_COOKIE_NAME } from "@/lib/session";

function redirectTo(path: string) {
  return NextResponse.redirect(path, { status: 307 });
}

export async function POST() {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
  return redirectTo("/login");
}

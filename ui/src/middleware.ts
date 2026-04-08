import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { SESSION_COOKIE_NAME } from "@/lib/session";

const ADMIN_ONLY_PATHS = [
  "/applications",
  "/audit",
  "/bundles",
  "/graph",
  "/policies",
  "/projects",
  "/users",
];

function readSession(request: NextRequest) {
  const raw = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) {
    return null;
  }

  try {
    const normalized = raw.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
    const decoded = atob(padded);
    const parsed = JSON.parse(decoded) as { userId?: string; role?: string };
    if (!parsed.userId || (parsed.role !== "admin" && parsed.role !== "user")) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function redirectTo(path: string) {
  return NextResponse.redirect(path, { status: 307 });
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/auth/") ||
    pathname.includes(".")
  ) {
    return NextResponse.next();
  }

  const session = readSession(request);

  if (!session && pathname !== "/login") {
    return redirectTo("/login");
  }

  if (session && pathname === "/login") {
    return redirectTo("/");
  }

  if (session?.role === "user" && ADMIN_ONLY_PATHS.some((entry) => pathname.startsWith(entry))) {
    return redirectTo("/");
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api).*)"],
};

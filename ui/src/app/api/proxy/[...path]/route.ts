import { getSession } from "@/lib/session";
import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL || "http://backend:8080";
const API_KEY = process.env.MKAUTH_API_KEY || "";

function isSelfScoped(path: string[], userId: string) {
  return path[0] === "users" && path[1] === userId && (path[2] === "access" || path[2] === "grants");
}

function isMemberAllowed(method: "GET" | "POST" | "PUT", path: string[], userId: string) {
  if (method === "GET") {
    if (path.length === 1 && (path[0] === "catalog" || path[0] === "applications" || path[0] === "requests")) {
      return true;
    }
    return isSelfScoped(path, userId);
  }

  if (method === "POST" && path.length === 1 && path[0] === "requests") {
    return true;
  }

  return false;
}

async function proxy(request: NextRequest, method: "GET" | "POST" | "PUT", path: string[]) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  if (session.role !== "admin" && !isMemberAllowed(method, path, session.id)) {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }

  const target = new URL(`${BACKEND_URL}/api/v1/${path.join("/")}`);
  target.search = request.nextUrl.search;

  const init: RequestInit = {
    method,
    headers: {
      "Authorization": `Bearer ${API_KEY}`,
    },
    cache: "no-store",
  };

  if (method !== "GET") {
    const body = await request.json();
    const payload = session.role === "admin"
      ? body
      : {
          ...body,
          requester_id: session.id,
        };

    init.headers = {
      ...init.headers,
      "Content-Type": "application/json",
    };
    init.body = JSON.stringify(payload);
  }

  try {
    const res = await fetch(target, init);
    const data = await res.json();
    if (session.role !== "admin" && method === "GET" && path.length === 1 && path[0] === "requests" && Array.isArray(data)) {
      return NextResponse.json(data.filter((entry) => entry?.requester_id === session.id), { status: res.status });
    }
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json({ error: "Backend unreachable" }, { status: 502 });
  }
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "GET", path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "POST", path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return proxy(request, "PUT", path);
}

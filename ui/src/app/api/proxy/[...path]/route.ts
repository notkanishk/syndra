import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL || "http://backend:8080";
const API_KEY = process.env.MKAUTH_API_KEY || "";

async function proxy(request: NextRequest, method: "GET" | "POST" | "PUT", path: string[]) {
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
    init.headers = {
      ...init.headers,
      "Content-Type": "application/json",
    };
    init.body = JSON.stringify(await request.json());
  }

  try {
    const res = await fetch(target, init);
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return NextResponse.json({ error: "Backend unreachable" }, { status: 502 });
  }
}

export async function GET(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, "GET", params.path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, "POST", params.path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, "PUT", params.path);
}

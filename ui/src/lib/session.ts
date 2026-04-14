import { cookies } from "next/headers";
import { nameToAvatar } from "@/lib/oidc";

export type SessionRole = "admin" | "user";

export interface SessionUser {
  id: string;
  name: string;
  email: string;
  title: string;
  team: string;
  status: string;
  avatar: string;
  location: string;
  role: SessionRole;
  // Phase 3: present for OIDC sessions, undefined for demo sessions
  accessToken?: string;
  sessionType: "demo" | "oidc";
}

// ---------------------------------------------------------------------------
// Internal cookie payload — discriminated union
// ---------------------------------------------------------------------------

interface DemoSessionCookie {
  type: "demo";
  userId: string;
  role: SessionRole;
}

export interface OidcSessionCookie {
  type: "oidc";
  accessToken: string;
  userId: string;
  role: SessionRole;
  name: string;
  email: string;
  expiresAt: number; // Unix seconds
}

type SessionCookiePayload = DemoSessionCookie | OidcSessionCookie;

// Legacy shape (no type field) — treated as demo
interface LegacySessionCookie {
  userId: string;
  role: SessionRole;
}

export const SESSION_COOKIE_NAME = "mkauth_session";

// ---------------------------------------------------------------------------
// Demo user catalog
// ---------------------------------------------------------------------------

const DEMO_USERS: SessionUser[] = [
  {
    id: "dev_admin",
    name: "Alice Rivera",
    email: "alice@makerspace.local",
    title: "Makerspace Director",
    team: "Operations",
    status: "active",
    avatar: "AR",
    location: "HQ",
    role: "admin",
    sessionType: "demo",
  },
  {
    id: "sam_student",
    name: "Sam Patel",
    email: "sam@makerspace.local",
    title: "Student Maker",
    team: "Members",
    status: "active",
    avatar: "SP",
    location: "Campus",
    role: "user",
    sessionType: "demo",
  },
  {
    id: "maya_staff",
    name: "Maya Chen",
    email: "maya@makerspace.local",
    title: "Lab Coordinator",
    team: "Staff",
    status: "active",
    avatar: "MC",
    location: "HQ",
    role: "admin",
    sessionType: "demo",
  },
  {
    id: "leo_mentor",
    name: "Leo Brooks",
    email: "leo@makerspace.local",
    title: "Laser Mentor",
    team: "Training",
    status: "active",
    avatar: "LB",
    location: "Annex",
    role: "user",
    sessionType: "demo",
  },
  {
    id: "ava_guest",
    name: "Ava Morgan",
    email: "ava@makerspace.local",
    title: "Visiting Artist",
    team: "Residency",
    status: "pending",
    avatar: "AM",
    location: "Studio",
    role: "user",
    sessionType: "demo",
  },
];

export function getDemoUsers(): SessionUser[] {
  return DEMO_USERS;
}

export function getDemoUser(userId: string): SessionUser | null {
  return DEMO_USERS.find((user) => user.id === userId) ?? null;
}

// ---------------------------------------------------------------------------
// Cookie encode / decode
// ---------------------------------------------------------------------------

function encodeSession(payload: SessionCookiePayload): string {
  return Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
}

function decodeSessionPayload(value: string): SessionCookiePayload | null {
  try {
    const decoded = Buffer.from(value, "base64url").toString("utf8");
    const parsed = JSON.parse(decoded) as Partial<SessionCookiePayload & LegacySessionCookie>;

    if (parsed.type === "oidc") {
      if (
        typeof parsed.accessToken !== "string" ||
        typeof parsed.userId !== "string" ||
        (parsed.role !== "admin" && parsed.role !== "user") ||
        typeof parsed.name !== "string" ||
        typeof parsed.email !== "string" ||
        typeof parsed.expiresAt !== "number"
      ) {
        return null;
      }
      return parsed as OidcSessionCookie;
    }

    // type === "demo" or legacy (no type field)
    if (!parsed.userId || (parsed.role !== "admin" && parsed.role !== "user")) {
      return null;
    }
    return { type: "demo", userId: parsed.userId, role: parsed.role };
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Public session factories
// ---------------------------------------------------------------------------

export function createSessionValue(userId: string): string | null {
  const user = getDemoUser(userId);
  if (!user) return null;
  return encodeSession({ type: "demo", userId: user.id, role: user.role });
}

export function createOidcSessionValue(payload: OidcSessionCookie): string {
  return encodeSession(payload);
}

/** Returns the bearer token for backend requests, or undefined for demo sessions. */
export function getSessionToken(user: SessionUser): string | undefined {
  return user.sessionType === "oidc" ? user.accessToken : undefined;
}

// ---------------------------------------------------------------------------
// getSession — resolves cookie to SessionUser
// ---------------------------------------------------------------------------

export async function getSession(): Promise<SessionUser | null> {
  const cookieStore = await cookies();
  const raw = cookieStore.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) return null;

  const payload = decodeSessionPayload(raw);
  if (!payload) return null;

  if (payload.type === "oidc") {
    // Reject expired tokens before they reach the backend
    if (Date.now() / 1000 > payload.expiresAt) return null;

    return {
      id: payload.userId,
      name: payload.name,
      email: payload.email,
      title: "",
      team: "",
      status: "active",
      location: "",
      avatar: nameToAvatar(payload.name),
      role: payload.role,
      accessToken: payload.accessToken,
      sessionType: "oidc",
    };
  }

  // Demo session
  const user = getDemoUser(payload.userId);
  if (!user || user.role !== payload.role) return null;
  return user;
}

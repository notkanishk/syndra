import { cookies } from "next/headers";

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
}

interface SessionCookie {
  userId: string;
  role: SessionRole;
}

export const SESSION_COOKIE_NAME = "mkauth_session";

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
  },
];

function encodeSession(session: SessionCookie) {
  return Buffer.from(JSON.stringify(session), "utf8").toString("base64url");
}

function decodeSession(value: string): SessionCookie | null {
  try {
    const decoded = Buffer.from(value, "base64url").toString("utf8");
    const parsed = JSON.parse(decoded) as SessionCookie;
    if (!parsed.userId || (parsed.role !== "admin" && parsed.role !== "user")) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function getDemoUsers() {
  return DEMO_USERS;
}

export function getDemoUser(userId: string) {
  return DEMO_USERS.find((user) => user.id === userId) ?? null;
}

export function createSessionValue(userId: string) {
  const user = getDemoUser(userId);
  if (!user) {
    return null;
  }

  return encodeSession({ userId: user.id, role: user.role });
}

export async function getSession() {
  const cookieStore = await cookies();
  const raw = cookieStore.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) {
    return null;
  }

  const parsed = decodeSession(raw);
  if (!parsed) {
    return null;
  }

  const user = getDemoUser(parsed.userId);
  if (!user || user.role !== parsed.role) {
    return null;
  }

  return user;
}


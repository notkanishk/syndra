// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

import LandingPage from "@/app/page";
import { DOOR_COOKIE, doorCookie, doorFor } from "@/lib/motion/door";
import type { SessionUser } from "@/lib/session";

// A-24 §3.2: the door is decided by the server, from the sign-in's own cookie, so the first HTML already
// carries it and the page never shows itself and then hides for it.

const jar = vi.hoisted(() => ({ door: undefined as string | undefined, session: null as unknown }));
vi.mock("next/headers", () => ({
  cookies: async () => ({ get: (name: string) => (name === "syndra_door" && jar.door !== undefined ? { name, value: jar.door } : undefined) }),
}));
vi.mock("@/lib/session", () => ({ getSession: async () => jar.session }));
vi.mock("@/components/home/Home", () => ({ Home: () => null }));
vi.mock("@/components/member/MemberAccess", () => ({ MemberAccess: () => null }));

const operator = { id: "op", role: "admin", signedInAt: "2026-11-19T08:00:00.000Z" } as SessionUser;

beforeEach(() => {
  jar.door = undefined;
  jar.session = operator;
});

describe("the landing page decides the door before the first paint", () => {
  it("renders Home with the door when the sign-in's own cookie is there", async () => {
    jar.door = operator.signedInAt;
    const page = (await LandingPage()) as { props: { door?: boolean } };
    expect(page.props.door).toBe(true);
  });

  it("renders Home without it when there is no cookie, or it names another sign-in", async () => {
    expect(((await LandingPage()) as { props: { door?: boolean } }).props.door).toBe(false);
    jar.door = "2026-11-18T08:00:00.000Z";
    expect(((await LandingPage()) as { props: { door?: boolean } }).props.door).toBe(false);
  });

  it("is a short-lived, readable cookie naming an instant, and nothing else", () => {
    const cookie = doorCookie(operator.signedInAt!);
    expect(cookie).toMatchObject({ name: DOOR_COOKIE, value: operator.signedInAt, httpOnly: false, sameSite: "lax", path: "/" });
    expect(cookie.maxAge).toBeLessThanOrEqual(60);
    expect(doorFor(undefined, operator.signedInAt)).toBe(false);
    expect(doorFor(operator.signedInAt, undefined)).toBe(false);
  });
});

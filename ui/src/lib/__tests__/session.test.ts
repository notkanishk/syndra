import { describe, it, expect, expectTypeOf, vi, beforeEach, afterEach } from "vitest";
import {
  createSessionValue,
  getAccessToken,
  getDemoUser,
  getDemoUsers,
  createOidcSessionValue,
  getSession,
  SESSION_COOKIE_NAME,
  type SessionUser,
} from "@/lib/session";

// Sessions are HMAC-signed (SC4); a secret must be present for encode/decode.
process.env.SESSION_SECRET = "test-session-secret";

// Extracts the JSON payload from a signed `payload.hmac` cookie value.
function decodePayload(value: string): Record<string, unknown> {
  return JSON.parse(Buffer.from(value.split(".")[0], "base64url").toString("utf8"));
}

// In-test cookie store. Mutated by beforeEach in the getSession suite below.
let mockCookieValue: string | undefined;
vi.mock("next/headers", () => ({
  cookies: async () => ({
    get: (name: string) =>
      name === SESSION_COOKIE_NAME && mockCookieValue !== undefined
        ? { value: mockCookieValue }
        : undefined,
  }),
}));

describe("getDemoUsers", () => {
  it("returns all 5 demo users", () => {
    const users = getDemoUsers();
    expect(users).toHaveLength(5);
    expect(users.map((u) => u.id)).toContain("dev_admin");
    expect(users.map((u) => u.id)).toContain("rey_student");
  });
});

describe("getDemoUser", () => {
  it("returns a known user by ID", () => {
    const user = getDemoUser("dev_admin");
    expect(user).not.toBeNull();
    expect(user!.name).toBe("Becky Lynch");
    expect(user!.role).toBe("admin");
    expect(user!.sessionType).toBe("demo");
  });

  it("returns null for unknown ID", () => {
    expect(getDemoUser("nonexistent")).toBeNull();
  });
});

describe("createSessionValue", () => {
  it("encodes a demo session as base64url JSON", () => {
    const value = createSessionValue("dev_admin");
    expect(value).not.toBeNull();

    // Decode and verify the payload
    const decoded = decodePayload(value!);
    expect(decoded).toEqual({
      type: "demo",
      userId: "dev_admin",
      role: "admin",
      signedInAt: expect.any(String),
    });
  });

  it("returns null for unknown user", () => {
    expect(createSessionValue("fake_user")).toBeNull();
  });
});

describe("createOidcSessionValue", () => {
  it("encodes an OIDC session with all required fields", () => {
    const payload = {
      type: "oidc" as const,
      accessToken: "eyJhbGciOiJSUzI1NiJ9.test",
      userId: "zitadel-user-123",
      role: "admin" as const,
      name: "Test User",
      email: "test@example.com",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };

    const value = createOidcSessionValue(payload);
    const decoded = decodePayload(value!);

    expect(decoded.type).toBe("oidc");
    expect(decoded.accessToken).toBe(payload.accessToken);
    expect(decoded.userId).toBe(payload.userId);
    expect(decoded.expiresAt).toBe(payload.expiresAt);
  });
});

describe("getSession", () => {
  const originalDomain = process.env.ZITADEL_DOMAIN;

  beforeEach(() => {
    mockCookieValue = undefined;
  });

  afterEach(() => {
    if (originalDomain === undefined) delete process.env.ZITADEL_DOMAIN;
    else process.env.ZITADEL_DOMAIN = originalDomain;
  });

  it("resolves a demo cookie when ZITADEL_DOMAIN is unset", async () => {
    delete process.env.ZITADEL_DOMAIN;
    mockCookieValue = createSessionValue("dev_admin")!;
    const session = await getSession();
    expect(session?.id).toBe("dev_admin");
    expect(session?.sessionType).toBe("demo");
  });

  it("rejects a demo cookie when ZITADEL_DOMAIN is set (defense-in-depth)", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createSessionValue("dev_admin")!;
    const session = await getSession();
    expect(session).toBeNull();
  });

  it("accepts an OIDC cookie regardless of ZITADEL_DOMAIN", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "z-1",
      role: "admin",
      name: "Real User",
      email: "real@example.com",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.sessionType).toBe("oidc");
    expect(session?.id).toBe("z-1");
  });

  // Pages pass the session to client components, whose props are serialised
  // into the HTML. The token must not be in it, under any key.
  it("never carries the access token, so a page cannot serialise it", async () => {
    expectTypeOf<SessionUser>().not.toHaveProperty("accessToken");
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok-must-stay-server-side",
      userId: "z-1",
      role: "admin",
      name: "Real User",
      email: "real@example.com",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.sessionType).toBe("oidc");
    expect(session).not.toHaveProperty("accessToken");
    expect(JSON.stringify(session)).not.toContain("tok-must-stay-server-side");
    expect(await getAccessToken()).toBe("tok-must-stay-server-side");
  });

  it("getAccessToken is null with no session, an expired one, or a demo one", async () => {
    mockCookieValue = undefined;
    expect(await getAccessToken()).toBeNull();

    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok-expired",
      userId: "z-1",
      role: "admin",
      name: "Real User",
      email: "real@example.com",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) - 1,
    });
    expect(await getAccessToken()).toBeNull();
    expect(await getSession()).toBeNull();

    delete process.env.ZITADEL_DOMAIN;
    mockCookieValue = createSessionValue("dev_admin")!;
    expect(await getAccessToken()).toBeNull();
  });

  it("returns null when no cookie is present", async () => {
    mockCookieValue = undefined;
    const session = await getSession();
    expect(session).toBeNull();
  });

  it("rejects a tampered cookie whose payload was edited after signing — SC4", async () => {
    const value = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Maven",
      email: "m@x.test",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const [, sig] = value.split(".");
    const escalated = Buffer.from(
      JSON.stringify({ ...decodePayload(value), role: "admin" }),
      "utf8",
    ).toString("base64url");
    mockCookieValue = `${escalated}.${sig}`;
    expect(await getSession()).toBeNull();
  });

  it("rejects an unsigned (pre-SC4 legacy) cookie", async () => {
    delete process.env.ZITADEL_DOMAIN;
    mockCookieValue = Buffer.from(
      JSON.stringify({ type: "demo", userId: "dev_admin", role: "admin" }),
      "utf8",
    ).toString("base64url");
    expect(await getSession()).toBeNull();
  });

  it("encodes title/team/status into the OIDC cookie payload", () => {
    const value = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Becky",
      email: "becky@x.test",
      title: "Director",
      team: "Ops",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const decoded = decodePayload(value!);
    expect(decoded.title).toBe("Director");
    expect(decoded.team).toBe("Ops");
    expect(decoded.status).toBe("active");
  });

  it("OIDC avatar falls back to email then userId when name is empty", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u-1",
      role: "user",
      name: "",
      email: "jacqueline.moore@example.edu",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.avatar).not.toBe("");
    expect(session?.avatar).toBe("JA"); // from email local-part "jacqueline.moore"
  });

  it("getSession surfaces title/team on OidcSessionUser", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Becky",
      email: "becky@x.test",
      title: "Director",
      team: "Ops",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.title).toBe("Director");
    expect(session?.team).toBe("Ops");
  });
});

describe("demo identities are gated at the source", () => {
  // vi.stubEnv rather than assigning process.env directly: process.env is
  // shared across everything running in this worker, and a hand-rolled restore
  // that misses a throw leaves the whole suite in demo-disabled mode.
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("returns nothing when a live identity provider is configured", () => {
    // The login page and getSession each check this too. This is the gate a
    // future caller cannot forget: there is no path to the array except these
    // two functions.
    vi.stubEnv("ZITADEL_DOMAIN", "id.example.org");
    expect(getDemoUsers()).toEqual([]);
    expect(getDemoUser("dev_admin")).toBeNull();
  });

  it("serves them in local development", () => {
    vi.stubEnv("ZITADEL_DOMAIN", "");
    expect(getDemoUsers().length).toBeGreaterThan(0);
    expect(getDemoUser("dev_admin")).not.toBeNull();
  });
});

describe("the first-visit flag", () => {
  const cookie = (firstVisit?: boolean, previousSignIn?: string, signedInAt?: string) =>
    createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "z-1",
      role: "user",
      name: "New Person",
      email: "n@x.test",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
      ...(firstVisit ? { firstVisit } : {}),
      ...(previousSignIn ? { previousSignIn } : {}),
      ...(signedInAt ? { signedInAt } : {}),
    });

  it("survives a refresh (every read of the same cookie), and is absent from any other session", async () => {
    mockCookieValue = cookie(true);
    expect((await getSession())?.firstVisit).toBe(true);
    expect((await getSession())?.firstVisit).toBe(true);
    mockCookieValue = cookie();
    expect((await getSession())?.firstVisit).toBeUndefined();
  });

  it("carries when they were last here, and is absent when the sign-in had none (A-20 P18)", async () => {
    mockCookieValue = cookie(false, "2026-11-03T09:00:00Z");
    expect((await getSession())?.previousSignIn).toBe("2026-11-03T09:00:00Z");
    mockCookieValue = cookie();
    expect((await getSession())?.previousSignIn).toBeUndefined();
  });

  it("carries the sign-in's own instant, the one the first arrival is kept to (A-24 §3.2)", async () => {
    mockCookieValue = cookie(false, undefined, "2026-11-19T08:00:00.000Z");
    expect((await getSession())?.signedInAt).toBe("2026-11-19T08:00:00.000Z");
  });

  it("stamps every sign-in where it becomes a session, the dev identity picker's too", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-19T09:30:00.000Z"));
    try {
      mockCookieValue = cookie();
      expect((await getSession())?.signedInAt).toBe("2026-11-19T09:30:00.000Z");
      vi.stubEnv("ZITADEL_DOMAIN", "");
      mockCookieValue = createSessionValue("dev_admin")!;
      const demo = await getSession();
      expect(demo?.sessionType).toBe("demo");
      expect(demo?.signedInAt).toBe("2026-11-19T09:30:00.000Z");
      // The catalogue's own record is never written to: each sign-in is its own copy.
      expect(getDemoUser("dev_admin")?.signedInAt).toBeUndefined();
    } finally {
      vi.unstubAllEnvs();
      vi.useRealTimers();
    }
  });

});

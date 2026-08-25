import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  createSessionValue,
  getDemoUser,
  getDemoUsers,
  createOidcSessionValue,
  getSession,
  SESSION_COOKIE_NAME,
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
    expect(users.map((u) => u.id)).toContain("sam_student");
  });
});

describe("getDemoUser", () => {
  it("returns a known user by ID", () => {
    const user = getDemoUser("dev_admin");
    expect(user).not.toBeNull();
    expect(user!.name).toBe("Alice Rivera");
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
      name: "Mallory",
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
      name: "Alice",
      email: "alice@x.test",
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
      email: "jane.doe@example.edu",
      title: "",
      team: "",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.avatar).not.toBe("");
    expect(session?.avatar).toBe("JA"); // from email local-part "jane.doe"
  });

  it("getSession surfaces title/team on OidcSessionUser", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Alice",
      email: "alice@x.test",
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

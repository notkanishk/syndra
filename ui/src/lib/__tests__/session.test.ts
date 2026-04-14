import { describe, it, expect } from "vitest";
import { createSessionValue, getDemoUser, getDemoUsers, createOidcSessionValue } from "@/lib/session";

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
    const decoded = JSON.parse(Buffer.from(value!, "base64url").toString("utf8"));
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
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };

    const value = createOidcSessionValue(payload);
    const decoded = JSON.parse(Buffer.from(value, "base64url").toString("utf8"));

    expect(decoded.type).toBe("oidc");
    expect(decoded.accessToken).toBe(payload.accessToken);
    expect(decoded.userId).toBe(payload.userId);
    expect(decoded.expiresAt).toBe(payload.expiresAt);
  });
});

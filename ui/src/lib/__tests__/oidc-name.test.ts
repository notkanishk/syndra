import { describe, expect, it } from "vitest";

import {
  extractSessionFields,
  nameFromEmail,
  resolveDisplayName,
} from "@/lib/oidc";

/**
 * The regression these tests exist for: a Zitadel access token carries profile
 * claims only when the instance is configured to inline userinfo. By default it
 * carries none, and `extractSessionFields` used to fall back to `claims.sub` —
 * so the shell header and the Home greeting rendered the operator's opaque
 * Zitadel id where their name belonged. `sub` must never surface as a name.
 */
describe("extractSessionFields — name", () => {
  const sub = "318742996051787783";

  it("never falls back to the subject id", () => {
    const fields = extractSessionFields({ sub, exp: 1 }, "admin");
    expect(fields.userId).toBe(sub);
    expect(fields.name).toBe("");
    expect(fields.name).not.toBe(sub);
  });

  it("ignores a whitespace-only name claim rather than treating it as present", () => {
    const fields = extractSessionFields({ sub, name: "   ", exp: 1 }, "admin");
    expect(fields.name).toBe("");
  });

  it("prefers the name claim, then preferred_username", () => {
    expect(extractSessionFields({ sub, name: "Ada Lovelace" }, "admin").name).toBe("Ada Lovelace");
    expect(extractSessionFields({ sub, preferred_username: "ada" }, "admin").name).toBe("ada");
    expect(
      extractSessionFields({ sub, name: "Ada Lovelace", preferred_username: "ada" }, "admin").name,
    ).toBe("Ada Lovelace");
  });

  it("still derives the admin role independent of the name claims", () => {
    const claims = { sub, "urn:zitadel:iam:org:project:roles": { admin: { org: "Makerspace" } } };
    expect(extractSessionFields(claims, "admin").role).toBe("admin");
    expect(extractSessionFields(claims, "admin").name).toBe("");
  });
});

describe("nameFromEmail", () => {
  it("title-cases a dotted local part", () => {
    expect(nameFromEmail("priya.sharma@example.org")).toBe("Priya Sharma");
  });

  it("handles underscores and hyphens", () => {
    expect(nameFromEmail("ada_lovelace@example.com")).toBe("Ada Lovelace");
    expect(nameFromEmail("ada-lovelace@example.com")).toBe("Ada Lovelace");
  });

  it("returns empty for a blank or malformed address", () => {
    expect(nameFromEmail("")).toBe("");
    expect(nameFromEmail("@example.com")).toBe("");
  });
});

describe("resolveDisplayName", () => {
  it("layers claim, then profile, then email", () => {
    expect(resolveDisplayName("Claim Name", "Profile Name", "e.mail@example.com")).toBe("Claim Name");
    expect(resolveDisplayName("", "Profile Name", "e.mail@example.com")).toBe("Profile Name");
    expect(resolveDisplayName("", "", "e.mail@example.com")).toBe("E Mail");
  });

  it("treats whitespace-only sources as absent", () => {
    expect(resolveDisplayName("  ", "  ", "ada.lovelace@example.com")).toBe("Ada Lovelace");
  });

  it("returns empty rather than inventing a name when nothing knows", () => {
    expect(resolveDisplayName("", "", "")).toBe("");
  });
});

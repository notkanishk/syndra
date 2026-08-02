import { describe, it, expect } from "vitest";
import { describeDuration, describeExpiry, humanizeKey, roleLabel } from "@/lib/format";

describe("roleLabel", () => {
  it("names a role as the pair it is", () => {
    expect(roleLabel("3D Lab", "member", "Member")).toBe("3D Lab / Member");
  });

  it("humanizes the key when the role has no display name", () => {
    expect(roleLabel("Door Access", "lab_pin")).toBe("Door Access / Lab Pin");
    expect(roleLabel("Door Access", "lab_pin", "")).toBe("Door Access / Lab Pin");
  });

  it("keeps the same key distinct across projects", () => {
    expect(roleLabel("3D Lab", "admin", "Administrator")).not.toBe(
      roleLabel("Metal Shop", "admin", "Administrator"),
    );
  });
});

describe("describeExpiry", () => {
  const now = new Date("2026-04-27T12:00:00Z");

  it("returns null for missing expiry", () => {
    expect(describeExpiry(null, now)).toBeNull();
    expect(describeExpiry(undefined, now)).toBeNull();
  });

  it("flags expired grants", () => {
    const past = new Date("2026-04-25T12:00:00Z").toISOString();
    const result = describeExpiry(past, now)!;
    expect(result.tone).toBe("expired");
  });

  it("flags critical (≤7 days)", () => {
    const soon = new Date("2026-04-30T12:00:00Z").toISOString();
    const result = describeExpiry(soon, now)!;
    expect(result.tone).toBe("critical");
    expect(result.daysLeft).toBe(3);
  });

  it("flags warning (8–14 days)", () => {
    const target = new Date("2026-05-08T12:00:00Z").toISOString();
    const result = describeExpiry(target, now)!;
    expect(result.tone).toBe("warning");
  });

  it("flags neutral (>14 days)", () => {
    const target = new Date("2026-06-01T12:00:00Z").toISOString();
    const result = describeExpiry(target, now)!;
    expect(result.tone).toBe("neutral");
  });
});

describe("humanizeKey", () => {
  it("converts snake_case to Title Case at word boundaries", () => {
    expect(humanizeKey("lab_pin")).toBe("Lab Pin");
    expect(humanizeKey("door-staff_entry")).toBe("Door Staff Entry");
  });
  it("returns empty for empty input", () => {
    expect(humanizeKey("")).toBe("");
  });
});

describe("describeDuration", () => {
  it("names the ask in days", () => {
    expect(describeDuration(30)).toBe("for 30 days");
    expect(describeDuration(1)).toBe("for 1 day");
  });

  // Zero is not "unspecified" — the backend reads it as a grant that never
  // lapses, so it is the one value that must never render as blank.
  it("says what an absent duration actually means", () => {
    expect(describeDuration(0)).toBe("no end date");
    expect(describeDuration(null)).toBe("no end date");
    expect(describeDuration(undefined)).toBe("no end date");
  });
});

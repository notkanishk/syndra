import { describe, it, expect } from "vitest";
import { describeExpiry, formatRoleRef, humanizeKey } from "@/lib/format";

const projects = [
  {
    id: "printing",
    name: "3D Lab",
    roles: [
      { key: "member", label: "Member" },
      { key: "calibrator", label: "Calibrator" },
    ],
  },
  { id: "doors", name: "Door Access" },
];

describe("formatRoleRef", () => {
  it("returns labeled and raw pair when both exist", () => {
    const { label, raw } = formatRoleRef("printing", "member", projects);
    expect(label).toBe("3D Lab · Member");
    expect(raw).toBe("printing:member");
  });

  it("falls back to humanized role key when label is missing", () => {
    const { label } = formatRoleRef("doors", "lab_pin", projects);
    expect(label).toBe("Door Access · Lab Pin");
  });

  it("falls back to project_id when project is unknown", () => {
    const { label } = formatRoleRef("unknown", "member", projects);
    expect(label).toBe("unknown · Member");
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

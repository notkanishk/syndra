import { describe, it, expect } from "vitest";
import {
  confirmedNote,
  describeDuration,
  describeExpiry,
  askLengthForMember,
  askLengthForOperator,
  formatShortDate,
  lengthWords,
  formatWhen,
  humanizeKey,
  personLine,
  roleLabel,
} from "@/lib/format";

describe("personLine", () => {
  it("reads title · team", () => {
    expect(personLine({ title: "Student", team: "Operations" })).toBe("Student · Operations");
  });

  it("leaves out an empty half, never a dash", () => {
    expect(personLine({ title: "Student", team: "" })).toBe("Student");
    expect(personLine({ title: "  ", team: "Operations" })).toBe("Operations");
    expect(personLine({ title: "", team: undefined })).toBe("");
  });
});

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
    expect(result.countdown).toBe("ended");
  });

  it("says ends, never expires", () => {
    const hours = new Date("2026-04-27T20:00:00Z").toISOString();
    expect(describeExpiry(hours, now)!.countdown).toBe("ends in 8 hours");
    const soon = new Date("2026-04-30T12:00:00Z").toISOString();
    expect(describeExpiry(soon, now)!.countdown).toBe("ends in 3 days");
    const later = new Date("2026-06-01T12:00:00Z").toISOString();
    expect(describeExpiry(later, now)!.countdown).toMatch(/^ends in \d+ days$/);
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

describe("confirmedNote", () => {
  // ErrNoObservation must never render as a confirmed zero — "not checked
  // yet" is a different fact from "checked, and found nobody".
  it("reads never-observed as not confirmed, not as zero", () => {
    expect(confirmedNote(4, undefined)).toBe("not confirmed");
    expect(confirmedNote(0, undefined)).toBe("not confirmed");
  });

  it("says confirmed, without repeating the number, when the two agree", () => {
    expect(confirmedNote(4, 4)).toBe("confirmed");
    expect(confirmedNote(0, 0)).toBe("confirmed");
  });

  // Recorded and confirmed are two different, both-useful facts — one can be
  // lower than the other without either being wrong.
  it("states the confirmed count on its own when it differs from recorded", () => {
    expect(confirmedNote(4, 2)).toBe("2 confirmed");
    expect(confirmedNote(2, 0)).toBe("0 confirmed");
  });
});

describe("formatShortDate", () => {
  // Noon UTC keeps the local calendar day the same in any timezone a test runs in.
  const now = new Date("2026-10-17T12:00:00Z");

  it("leaves the year off a date in this year", () => {
    expect(formatShortDate("2026-08-02T12:00:00Z", now)).toBe("2 Aug");
  });

  it("names the year of a date next year", () => {
    expect(formatShortDate("2027-01-18T12:00:00Z", now)).toBe("18 Jan 2027");
  });

  it("names the year of a date last year", () => {
    expect(formatShortDate("2025-12-18T12:00:00Z", now)).toBe("18 Dec 2025");
  });

  it("stays a dash for nothing", () => {
    expect(formatShortDate(null, now)).toBe("—");
    expect(formatShortDate("not a date", now)).toBe("—");
  });

  it("carries the year into a feed stamp too", () => {
    expect(formatWhen("2027-01-18T12:00:00Z", now)).toMatch(/^18 Jan 2027 /);
  });
});

describe("a pending ask's length", () => {
  const now = new Date("2026-10-17T12:00:00Z");

  it("says the common lengths as words", () => {
    expect(lengthWords(7)).toBe("a week");
    expect(lengthWords(30)).toBe("a month");
    expect(lengthWords(45)).toBe("45 days");
    expect(lengthWords(1)).toBe("1 day");
  });

  it("tells the member the length only, never a date", () => {
    expect(askLengthForMember(30)).toBe("For a month, from when it's approved.");
    expect(askLengthForMember(7)).toBe("For a week, from when it's approved.");
    expect(askLengthForMember(0)).toBe("With no end date.");
  });

  it("tells the operator the length and the date approving now would end it", () => {
    expect(askLengthForOperator(30, now)).toBe("For a month · until 16 Nov");
    expect(askLengthForOperator(null, now)).toBe("With no end date");
  });

  it("names the year when that date falls in the next one", () => {
    expect(askLengthForOperator(90, now)).toBe("For 90 days · until 15 Jan 2027");
  });
});

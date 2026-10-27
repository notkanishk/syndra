import { describe, it, expect } from "vitest";
import {
  expiryLabel,
  formatAgoShort,
  askLengthForMember,
  askLengthForOperator,
  formatShortDate,
  lengthWords,
  formatWhen,
  humanizeKey,
  personLine,
  roleLabel,
} from "@/lib/format";

describe("expiryLabel", () => {
  // Local noon, so calendar-day maths does not depend on the runner's time zone.
  const now = new Date(2026, 9, 27, 12, 0, 0);
  const at = (days: number, hour = 9) => new Date(2026, 9, 27 + days, hour, 0, 0).toISOString();

  it("never says '-2 days', '1 days' or '0 days left'", () => {
    for (const form of ["short", "left", "long"] as const) {
      for (let d = -5; d <= 40; d++) {
        const label = expiryLabel(at(d), form, now)!;
        expect(label, `${form} ${d}`).not.toMatch(/-\d|\b1 days|\b0 days/);
      }
    }
  });

  it("says ended for anything already past, even by minutes", () => {
    expect(expiryLabel(at(-2), "long", now)).toBe("Ended");
    expect(expiryLabel(at(0, 11), "left", now)).toBe("Ended");
    expect(expiryLabel(at(0, 11), "short", now)).toBe("Ended");
  });

  it("counts calendar days: later today is today, the next day is tomorrow", () => {
    expect(expiryLabel(at(0, 23), "long", now)).toBe("Ends today");
    expect(expiryLabel(at(0, 23), "left", now)).toBe("Ends today");
    expect(expiryLabel(at(0, 23), "short", now)).toBe("Today");
    // Eleven hours away but on tomorrow's date, which is how the date beside it reads.
    expect(expiryLabel(at(1, 6), "long", now)).toBe("Ends tomorrow");
  });

  it("pluralises, in each form", () => {
    expect(expiryLabel(at(1), "short", now)).toBe("1 day");
    expect(expiryLabel(at(1), "left", now)).toBe("1 day left");
    expect(expiryLabel(at(3), "short", now)).toBe("3 days");
    expect(expiryLabel(at(3), "left", now)).toBe("3 days left");
    expect(expiryLabel(at(3), "long", now)).toBe("Ends in 3 days");
  });

  it("is null when there is no end to say", () => {
    expect(expiryLabel(null)).toBeNull();
    expect(expiryLabel(undefined)).toBeNull();
    expect(expiryLabel("not a date")).toBeNull();
  });
});

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

describe("humanizeKey", () => {
  it("converts snake_case to Title Case at word boundaries", () => {
    expect(humanizeKey("lab_pin")).toBe("Lab Pin");
    expect(humanizeKey("door-staff_entry")).toBe("Door Staff Entry");
  });
  it("returns empty for empty input", () => {
    expect(humanizeKey("")).toBe("");
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

  it("names the date the ask itself named, not one counted from now", () => {
    // Approved late, an "until the end of term" ask still ends on the term's last day.
    expect(askLengthForOperator(60, now, "2026-12-31T00:00:00Z")).toBe("For 60 days · until 31 Dec");
  });

  it("names the year when that date falls in the next one", () => {
    expect(askLengthForOperator(90, now)).toBe("For 90 days · until 15 Jan 2027");
  });
});

describe("formatAgoShort", () => {
  const now = new Date("2026-10-17T12:00:00Z");
  it("is as short as a note seat needs", () => {
    expect(formatAgoShort("2026-10-17T11:59:40Z", now)).toBe("just now");
    expect(formatAgoShort("2026-10-17T11:30:00Z", now)).toBe("30m ago");
    expect(formatAgoShort("2026-10-17T10:00:00Z", now)).toBe("2h ago");
    expect(formatAgoShort("2026-10-14T12:00:00Z", now)).toBe("3d ago");
    expect(formatAgoShort("2026-08-01T12:00:00Z", now)).toBe("1 Aug");
    expect(formatAgoShort(undefined, now)).toBe("");
  });
});

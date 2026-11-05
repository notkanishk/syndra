import { describe, expect, it } from "vitest";

import { greeting, isQuietHour, lastHere } from "@/lib/greeting";
import { termDayLine } from "@/lib/term-dates";

const at = (hour: number) => new Date(2026, 10, 5, hour, 0, 0);

describe("greeting", () => {
  it("is Morning before 12, Afternoon before 17, Evening after", () => {
    expect([5, 11, 12, 16, 17, 23].map((h) => greeting(at(h)))).toEqual([
      "Morning",
      "Morning",
      "Afternoon",
      "Afternoon",
      "Evening",
      "Evening",
    ]);
  });
});

describe("quiet hours", () => {
  it("run from 22:00 up to 05:00, and greet as Evening throughout", () => {
    const quiet = [22, 23, 0, 1, 2, 4].map((h) => [isQuietHour(at(h)), greeting(at(h))]);
    expect(quiet.every(([q, g]) => q === true && g === "Evening")).toBe(true);
    // The edges: 21:59 is not quiet, 05:00 is not quiet and is Morning.
    expect(isQuietHour(new Date(2026, 10, 5, 21, 59))).toBe(false);
    expect(isQuietHour(new Date(2026, 10, 5, 4, 59))).toBe(true);
    expect(isQuietHour(at(5))).toBe(false);
    expect(greeting(at(5))).toBe("Morning");
    expect(greeting(at(21))).toBe("Evening");
  });
});

describe("termDayLine", () => {
  const terms = [
    { name: "Spring", start: "2026-01-05", end: "2026-04-30" },
    { name: "Monsoon term", start: "2026-08-03", end: "2026-11-27" },
  ];
  it("speaks on a term's first and last day only", () => {
    expect(termDayLine(terms, new Date(2026, 0, 5))).toBe("First day of Spring term");
    expect(termDayLine(terms, new Date(2026, 3, 30))).toBe("Last day of Spring term");
    expect(termDayLine(terms, new Date(2026, 10, 27))).toBe("Last day of Monsoon term");
    expect(termDayLine(terms, new Date(2026, 1, 1))).toBeNull();
    expect(termDayLine([], new Date(2026, 0, 5))).toBeNull();
  });
});

describe("lastHere (A-20 P18)", () => {
  // Wednesday 4 November 2026, local noon.
  const now = new Date(2026, 10, 4, 12, 0, 0);
  const at = (day: number, month = 10) => new Date(2026, month, day, 9, 0, 0).toISOString();

  it("says nothing for today, and for what it cannot read", () => {
    expect(lastHere(at(4), now)).toBeNull();
    expect(lastHere(undefined, now)).toBeNull();
    expect(lastHere("not a date", now)).toBeNull();
  });
  it("says yesterday, then the weekday under a week", () => {
    expect(lastHere(at(3), now)).toEqual({ since: "yesterday", on: "yesterday" });
    expect(lastHere(at(2), now)).toEqual({ since: "Monday", on: "on Monday" });
    expect(lastHere(at(29, 9), now)).toEqual({ since: "Thursday", on: "on Thursday" });
  });
  it("says a date from a week on", () => {
    expect(lastHere(at(28, 9), now)).toEqual({ since: "28 Oct", on: "on 28 Oct" });
  });
});

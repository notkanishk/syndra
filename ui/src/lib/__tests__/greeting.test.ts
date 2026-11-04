import { describe, expect, it } from "vitest";

import { greeting } from "@/lib/greeting";
import { termDayLine } from "@/lib/term-dates";

const at = (hour: number) => new Date(2026, 10, 5, hour, 0, 0);

describe("greeting", () => {
  it("is Morning before 12, Afternoon before 17, Evening after", () => {
    expect([0, 11, 12, 16, 17, 23].map((h) => greeting(at(h)))).toEqual([
      "Morning",
      "Morning",
      "Afternoon",
      "Afternoon",
      "Evening",
      "Evening",
    ]);
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

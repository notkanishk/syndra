import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { addDays, civilDay, daysBetween, daysFromToday, endAfterDaysISO, endOfDayISO, isSameDay } from "@/lib/time";

describe("the makerspace's day", () => {
  it("is the IST day, not the UTC one: 23:00 UTC is 04:30 the next morning", () => {
    expect(civilDay(new Date("2026-11-04T23:00:00Z"))).toBe("2026-11-05");
    expect(civilDay(new Date("2026-11-04T18:29:59Z"))).toBe("2026-11-04");
    expect(isSameDay(new Date("2026-11-04T18:30:00Z"), new Date("2026-11-05T18:29:59Z"))).toBe(true);
  });

  it("counts calendar days with no zone in the arithmetic", () => {
    expect(addDays("2026-12-25", 10)).toBe("2027-01-04");
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
    expect(daysBetween("2026-11-05", "2026-11-30")).toBe(25);
    expect(daysFromToday("2026-11-06", new Date("2026-11-05T18:31:00Z"))).toBe(0); // already the 6th in IST
  });
});

describe("an end date", () => {
  it("holds through the whole of its day in IST, never UTC midnight (05:30 IST)", () => {
    expect(endOfDayISO("2026-11-30")).toBe("2026-11-30T18:29:59.000Z");
  });

  it("'N days' ends as the Nth calendar day after today does", () => {
    // 02:00 IST on 6 Nov is still 5 Nov in UTC: the count starts from the 6th.
    expect(endAfterDaysISO(7, new Date("2026-11-05T20:30:00Z"))).toBe("2026-11-13T18:29:59.000Z");
    expect(endAfterDaysISO(30, new Date("2026-11-05T06:00:00Z"))).toBe("2026-12-05T18:29:59.000Z");
  });
});

/**
 * The guard. A date-only value ("YYYY-MM-DD", what a date picker gives) is a
 * makerspace day; turning it into an instant goes through lib/time.ts. Each
 * pattern below is a way this product already got it wrong:
 *   - `new Date(date).toISOString()` on a picker value is UTC midnight, so
 *     "until 30 Nov" lifted at 05:30 IST on 30 Nov;
 *   - `new Date(\`${day}T…\`)` is the browser's zone, not the makerspace's;
 *   - `toISOString().slice(0, 10)` is the UTC day of an instant.
 */
const SRC = join(import.meta.dirname, "../..");
const BANNED: Array<[string, RegExp]> = [
  ["a picker value through toISOString()", /new Date\((date|day|ymd|custom|reviewDate|endsOn|reviewOn|until)\)\.toISOString\(\)/],
  ["a literal date-only string as a Date", /new Date\(\s*["'`]\d{4}-\d{2}-\d{2}["'`]\s*\)/],
  ["a day string read in the browser's zone", /new Date\(`\$\{[^}]+\}T/],
  ["the UTC day of an instant", /toISOString\(\)\s*\.\s*(slice\(0,\s*10\)|split\(["']T["']\))/],
];

function* sources(dir: string): Generator<string> {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      if (name !== "__tests__" && name !== "node_modules") yield* sources(path);
    } else if (/\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) && !path.endsWith("lib/time.ts")) yield path;
  }
}

describe("days and end dates", () => {
  it("only become instants through lib/time.ts", () => {
    const found: string[] = [];
    for (const file of sources(SRC)) {
      readFileSync(file, "utf8")
        .split("\n")
        .forEach((line, i) => {
          for (const [why, re] of BANNED) if (re.test(line)) found.push(`${file.slice(SRC.length + 1)}:${i + 1} ${why}`);
        });
    }
    expect(found).toEqual([]);
  });
});

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { formatClock, formatClockSeconds } from "@/lib/format";
import {
  addDays,
  civilDay,
  daysBetween,
  daysFromToday,
  endAfterDaysISO,
  endOfDayISO,
  isSameDay,
  makerspaceZone,
  setMakerspaceZone,
  zoneTag,
} from "@/lib/time";

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

/**
 * The zone is the backend's TZ (clock.Name), handed over by the layout. Every
 * reader (day, end of day, clock) follows it; none keeps a zone of its own.
 */
describe("the zone the backend names", () => {
  // The suite's own zone is test config (vitest.config.ts); the product writes none.
  beforeEach(() => setMakerspaceZone("Asia/Kolkata"));
  afterEach(() => setMakerspaceZone("Asia/Kolkata"));

  it("moves the day, the end of a day and the clock together", () => {
    setMakerspaceZone("America/Santiago"); // UTC-3 in November (DST)
    const at = new Date("2026-11-05T01:00:00Z"); // 22:00 on the 4th there, 06:30 on the 5th in IST
    expect(civilDay(at)).toBe("2026-11-04");
    expect(endOfDayISO("2026-11-30")).toBe("2026-12-01T02:59:59.000Z");
    expect(formatClock(at.toISOString())).toBe("22:00");
    setMakerspaceZone("UTC");
    expect(formatClock(at.toISOString())).toBe("01:00");
  });

  it("shows UTC, labelled, when the backend's zone is unknown: never a guessed zone", () => {
    for (const unknown of [undefined, null, "", "Not/AZone"]) {
      setMakerspaceZone("Europe/London");
      setMakerspaceZone(unknown);
      expect(makerspaceZone()).toBe("UTC");
      expect(zoneTag()).toBe(" UTC");
      expect(formatClock("2026-11-05T01:00:00Z")).toBe("01:00 UTC");
      expect(formatClockSeconds("2026-11-05T01:00:09Z")).toBe("01:00:09 UTC");
    }
  });

  it("says nothing extra when the zone is known, even UTC itself", () => {
    setMakerspaceZone("UTC");
    expect(zoneTag()).toBe("");
    expect(formatClock("2026-11-05T01:00:00Z")).toBe("01:00");
  });
});

/**
 * A time shown with no zone named is read in the browser's, so an admin abroad
 * saw different hours from the rest of the makerspace. Every toLocaleTimeString,
 * toLocaleDateString and Intl.DateTimeFormat call names its zone (timeZone, or
 * inZone() which is makerspaceZone()).
 */
describe("a time is never shown in the browser's zone", () => {
  // A zone is named when timeZone has a value (not `timeZone: undefined`, which
  // is the browser's zone again) or the call spreads inZone().
  const namesAZone = (call: string) => /inZone\(\)/.test(call) || /timeZone\b(?!\s*:\s*undefined\b)/.test(call);
  it("recognises a named zone, and refuses undefined as one", () => {
    expect(namesAZone('("en-GB", { timeZone: "UTC" })')).toBe(true);
    expect(namesAZone('("en-GB", { ...inZone(), hour: "2-digit" })')).toBe(true);
    expect(namesAZone('("en-GB", { timeZone: undefined })')).toBe(false);
    expect(namesAZone('("en-GB", { timeZone:undefined, hour12: false })')).toBe(false);
    expect(namesAZone('("en-GB", { hour12: false })')).toBe(false);
  });
  const CALL = /(toLocale(?:Time|Date)String|Intl\.DateTimeFormat)\s*\(/g;
  it("names a zone at every call", () => {
    const found: string[] = [];
    for (const file of sources(SRC)) {
      const text = readFileSync(file, "utf8");
      for (const m of text.matchAll(CALL)) {
        let depth = 1;
        let end = m.index + m[0].length;
        for (; end < text.length && depth > 0; end++) {
          if (text[end] === "(") depth++;
          else if (text[end] === ")") depth--;
        }
        if (!namesAZone(text.slice(m.index, end))) {
          found.push(`${file.slice(SRC.length + 1)}:${text.slice(0, m.index).split("\n").length} ${m[1]} without a zone`);
        }
      }
    }
    expect(found).toEqual([]);
  });
});

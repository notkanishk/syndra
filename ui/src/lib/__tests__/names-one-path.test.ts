import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * One path for a name. A role's label is decided in lib/names.ts (`roleName`),
 * so no file writes the chain `display_name || humanizeKey(key)` itself; and a
 * list of names is sorted by lib/sort.ts, so no file sorts names with a bare
 * `localeCompare` (which reads "Lab 10" before "Lab 2" and, without options,
 * "b" after "Z").
 */
const SRC = join(__dirname, "..", "..");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) {
      if (entry === "__tests__" || entry === "node_modules") continue;
      walk(p, out);
    } else if (/\.(ts|tsx)$/.test(p) && !/\.test\.tsx?$/.test(p)) out.push(p);
  }
  return out;
}

const files = walk(SRC).map((p) => ({ rel: relative(SRC, p), text: readFileSync(p, "utf8") }));

describe("one name path", () => {
  it("no inline role-label chain outside lib/names.ts", () => {
    const chain = /(?:display_name|label)\s*(?:\|\||\?\?)\s*humanizeKey\(/;
    const bad = files
      .filter((f) => f.rel !== join("lib", "names.ts") && chain.test(f.text.replace(/\s+/g, " ")))
      .map((f) => f.rel);
    expect(bad).toEqual([]);
  });

  it("one phrase for a person nobody can name (lib/names.ts personOr)", () => {
    // "They", "Somebody", "Someone" or a literal "this person" as a name's fallback.
    const fallback = /(?:\?\?|\|\|)\s*"(?:They|Somebody|Someone|this person)"|fallback="(?:Somebody|Someone|They)"/;
    const bad: string[] = [];
    for (const f of files) {
      if (f.rel === join("lib", "names.ts")) continue;
      f.text.split("\n").forEach((line, i) => {
        if (fallback.test(line)) bad.push(`${f.rel}:${i + 1}`);
      });
    }
    expect(bad).toEqual([]);
  });

  it("no bare localeCompare on names (time fields compare as text on purpose)", () => {
    // A line that sorts timestamps (`_at`, `.start`, `joined`, `expires`, `earliest_end`) is ISO text, where
    // byte order is the right order. Anything else goes through lib/sort.ts.
    const time = /_at\b|\.start\b|joined|expires|earliest_end/;
    const bad: string[] = [];
    for (const f of files) {
      if (f.rel === join("lib", "sort.ts")) continue;
      f.text.split("\n").forEach((line, i) => {
        if (line.includes("localeCompare(") && !time.test(line)) bad.push(`${f.rel}:${i + 1}`);
      });
    }
    expect(bad).toEqual([]);
  });
});

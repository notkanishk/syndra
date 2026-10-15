import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * One control, one primitive. A checkbox drawn by hand on each screen drifted
 * to three sizes; a select or a pill button drawn by hand is the same drift
 * waiting. Outside components/ui, controls come from the primitives.
 */

const SRC = join(import.meta.dirname, "../../..");
const PRIMITIVES = join(SRC, "components/ui");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      if (entry !== "__tests__") walk(path, out);
    } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
      out.push(path);
    }
  }
  return out;
}

const RULES: Array<[string, RegExp, string]> = [
  ["a raw <select>", /<select\b/, "Select"],
  ["a raw <textarea>", /<textarea\b/, "Textarea"],
  ["a raw checkbox or radio", /type="(?:checkbox|radio)"/, "Checkbox or Radio from components/ui/Choice"],
  ["a <button> styled as a pill", /<button\b[^>]*className="[^"]*\brounded-pill\b[^"]*\bborder\b/, "Button"],
  ["a hand-written \"… →\" link", /→\s*<\/Link>/, "ArrowLink (it draws the arrow)"],
  // A dialog that returns null while closed keeps its state, and reopens on
  // its last result. It mounts only while open instead.
  ["a dialog that stays mounted while closed", /if \(!open\) return null;/, "mountedWhileOpen from components/ui/Modal"],
];

describe("controls come from the primitives", () => {
  const files = walk(SRC).filter((file) => !file.startsWith(PRIMITIVES));
  for (const [what, pattern, use] of RULES) {
    it(`never draws ${what} outside components/ui`, () => {
      const offenders = files
        .filter((file) => pattern.test(readFileSync(file, "utf8").replace(/\n\s*/g, " ")))
        .map((file) => relative(SRC, file));
      expect(offenders, `use ${use}`).toEqual([]);
    });
  }
});

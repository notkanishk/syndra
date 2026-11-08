import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { FORECAST } from "@/lib/outcome";

/**
 * A-19 EOY-F9: the copy guide's forecast-tense table (section 5) and
 * `FORECAST` say the same words. If one moves the other fails, so the guide
 * is never a description of last month's code.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");

function forecastColumn(): string[] {
  const start = guide.indexOf("| Act | While it is in flight | When it has happened |");
  expect(start, "the forecast table is in section 5").toBeGreaterThan(-1);
  const lines = guide.slice(start).split("\n").slice(2); // past header and separator
  const end = lines.findIndex((line) => !line.startsWith("|"));
  // Two acts may share a forecast (Remove, and Revoke under Drift): each word once.
  return [...new Set(lines.slice(0, end).map((line) => line.split("|")[2].trim()))];
}

describe("the copy guide's forecast table", () => {
  it("lists exactly the words FORECAST says, in the same order", () => {
    expect(forecastColumn()).toEqual([
      FORECAST.approve,
      FORECAST.decline,
      FORECAST.extend,
      FORECAST.remove,
      FORECAST.revoke,
      FORECAST.apply,
      FORECAST.keep,
      FORECAST.lift,
      FORECAST.give,
      FORECAST.cancel,
      FORECAST.adopt,
      FORECAST.mark,
      FORECAST.replace,
      FORECAST.queued,
    ]);
  });

  it("covers every FORECAST key, so a new forecast cannot skip the guide", () => {
    expect(forecastColumn().sort()).toEqual(Object.values(FORECAST).sort());
  });

  it("says each as the future, and keeps the guide's own rules: no em dash, no exclamation", () => {
    for (const word of Object.values(FORECAST)) expect(word).toMatch(/^Will /);
    for (const word of forecastColumn()) {
      expect(word).not.toMatch(/[—!]/);
    }
  });
});

import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { SRC, block, colour, ratio } from "./cssTokens";

/**
 * The statistics charts' colours (dataviz): the series is --accent, a gap's
 * hatch is --chart-hatch, both authored in full for both themes in
 * globals.css, and read nowhere as a hex. A stripe that cannot be seen
 * would draw a gap as nothing, which reads as a zero.
 */
// The violet ramp is theme-independent (its own :root block); both themes point --accent at it.
const RAMP = block(":root {\n    --violet-600");
const THEMES = {
  dark: { ...RAMP, ...block(':root,\n  [data-theme="dark"] {') },
  light: { ...RAMP, ...block('[data-theme="light"] {') },
};
const GROUNDS = ["--surface-1", "--canvas", "--surface-0"];

describe("the chart tokens, both themes", () => {
  for (const [theme, vars] of Object.entries(THEMES)) {
    it(`${theme} authors the hatch and the series, and the hatch clears 3:1 on every ground a card sits on`, () => {
      expect(vars["--chart-hatch"], "--chart-hatch").toBeDefined();
      const failures: string[] = [];
      for (const token of ["--chart-hatch", "--accent"]) {
        for (const g of GROUNDS) {
          const r = ratio(colour(vars, token), colour(vars, g));
          if (r < 3) failures.push(`${token} on ${g}: ${r.toFixed(2)}`);
        }
      }
      expect(failures).toEqual([]);
    });
  }

  it("the hatch is not the grid line: a gap must stand out from the baseline it sits on", () => {
    for (const vars of Object.values(THEMES)) {
      expect(ratio(colour(vars, "--chart-hatch"), colour(vars, "--surface-1"))).toBeGreaterThan(ratio(colour(vars, "--line-strong"), colour(vars, "--surface-1")));
    }
  });

  it("no statistics component writes a colour of its own", () => {
    const dir = resolve(SRC, "components/usage");
    const found: string[] = [];
    for (const f of readdirSync(dir).filter((n) => /\.tsx?$/.test(n))) {
      const code = readFileSync(resolve(dir, f), "utf8").replace(/\/\*[\s\S]*?\*\/|^\s*\/\/.*$/gm, "");
      for (const m of code.matchAll(/#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(/g)) found.push(`${f}: ${m[0]}`);
    }
    expect(found).toEqual([]);
  });

  it("the chart reads the tokens by name, so the themes decide the colour", () => {
    const spark = readFileSync(resolve(SRC, "components/usage/Spark.tsx"), "utf8");
    expect(spark).toContain("var(--chart-hatch)");
    expect(spark).toContain("var(--accent)");
  });
});

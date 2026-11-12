import { readFileSync, readdirSync, statSync } from "node:fs";

import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { SRC, block, colour, over, ratio } from "./cssTokens";

/**
 * Quiet text clears AA on every ground it can sit on (A-09 §1–2).
 *
 * Measured the way A-09 measured it: WCAG 2 relative luminance, with the text's
 * alpha composited over the ground, and each ground also tried under the three
 * tints that sit beneath text — a pressed row, a chosen row and an amber Notice.
 * A token change that drops any pair under 4.5:1 fails here, by name.
 */

const THEMES = {
  dark: { vars: block(':root,\n  [data-theme="dark"] {'), floor: 0.56 },
  light: { vars: block('[data-theme="light"] {'), floor: 0.66 },
};
const GROUNDS = ["--ground", "--canvas", "--surface-0", "--surface-1", "--surface-2", "--rail"];
const TINTS = [null, "--tint-3", "--accent-soft", "--warn-soft"];

describe("quiet text", () => {
  for (const [theme, { vars, floor }] of Object.entries(THEMES)) {
    it(`clears 4.5:1 on every ${theme} ground, tints included`, () => {
      const failures: string[] = [];
      for (const token of ["--muted", "--faint", "--label"]) {
        for (const g of GROUNDS) {
          for (const t of TINTS) {
            const ground = t ? over(colour(vars, t), colour(vars, g)) : colour(vars, g);
            const r = ratio(colour(vars, token), ground);
            if (r < 4.5) failures.push(`${token} on ${g}${t ? ` + ${t}` : ""}: ${r.toFixed(2)}`);
          }
        }
      }
      expect(failures).toEqual([]);
    });

    it(`keeps ${theme} faint at or above its floor, and label as strong as faint`, () => {
      expect(colour(vars, "--faint")[3]).toBeGreaterThanOrEqual(floor);
      expect(colour(vars, "--muted")[3]).toBeGreaterThan(colour(vars, "--faint")[3]);
      expect(vars["--label"]).toBe("var(--faint)");
    });
  }

  it("lifts the doorway's own text to the same steps", () => {
    const door = block(":root {\n  /* Always-dark surface tokens.");
    const page = colour(door, "--door-page");
    for (const token of ["--door-muted", "--door-faint"]) {
      expect(ratio(colour(door, token), page), token).toBeGreaterThanOrEqual(4.5);
    }
    // The sign-in button's label while it waits, on that button's quiet fill.
    const quietFill = over(colour(door, "--door-fill-quiet"), page);
    expect(ratio(colour(door, "--door-label-quiet"), quietFill)).toBeGreaterThanOrEqual(4.5);
  });
});

function* sources(dir: string): Generator<string> {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      if (entry !== "__tests__" && entry !== "node_modules") yield* sources(path);
    } else if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) yield path;
  }
}

describe("text drawn with its own alpha", () => {
  // The light floor is the stricter one, and a class carries into both themes.
  it("never goes under 0.66 of ink", () => {
    const found: string[] = [];
    for (const file of sources(SRC)) {
      for (const m of Array.from(readFileSync(file, "utf8").matchAll(/\btext-ink\/(\d+)\b/g))) {
        if (Number(m[1]) < 66) found.push(`${file.replace(SRC, "src")}: ${m[0]}`);
      }
    }
    expect(found).toEqual([]);
  });

  it("never dims stale data: no 0.62 left anywhere", () => {
    const found: string[] = [];
    for (const file of sources(SRC)) {
      if (/opacity-\[0?\.62\]|opacity:\s*\w+\s*\?\s*0?\.62/.test(readFileSync(file, "utf8"))) found.push(file.replace(SRC, "src"));
    }
    expect(found).toEqual([]);
  });
});

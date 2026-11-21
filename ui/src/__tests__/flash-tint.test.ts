import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { SRC, braced, colour, css, over, ratio, themes } from "./cssTokens";

// A-25 Q10, owner's decision (21 Nov 2026, "okay with A"): `flash` stays a flat tint on the thing that
// changed. A cue that points at one thing is a tint and takes that thing's shape; a light has none.
describe("flash is a tint, not a light", () => {
  const keyframes = braced("@keyframes flash {");
  const utility = braced("@utility flash {");

  it("tints from --accent-soft and ends on the element's own ground, so nothing snaps at the end", () => {
    expect(keyframes).toMatch(/from \{\s*background-color: var\(--accent-soft\);\s*\}/);
    expect(keyframes).not.toMatch(/\bto\b|transparent/);
  });

  it("draws no pool, no registered property and no pseudo-element, and keeps its timing", () => {
    expect(utility).toMatch(/animation: flash var\(--duration-flash\) var\(--ease-flash\) 1;/);
    expect(utility).not.toMatch(/gradient|--pool|--flash|::before|::after/);
    expect(css).not.toMatch(/@property --flash|--flash-k/);
    expect(css).not.toMatch(/:where\([^)]*\.flash/);
  });

  it("is not stacked on \"New since you looked\", whose resting tint and words are the cue already", () => {
    const source = readFileSync(resolve(SRC, "components/ui/CancelQueuedStep.tsx"), "utf8");
    expect(source).toContain('g.fresh ? "bg-accent-soft" : ""');
    expect(source).not.toMatch(/\bflash\b/);
  });

  it("never takes text under 4.5:1 or a dot under 3:1 at its peak, on the canvas, a card and the rail, in both themes", () => {
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes())) {
      const soft = colour(vars, "--accent-soft");
      for (const ground of ["--canvas", "--surface-1", "--rail"]) {
        const tint = over(soft, colour(vars, ground));
        for (const token of ["--ink", "--muted", "--faint", "--accent-text", "--warn-text", "--danger-text"]) {
          const r = ratio(colour(vars, token), tint);
          if (r < 4.5) failures.push(`${theme} ${ground} ${token}: ${r.toFixed(2)}`);
        }
        for (const token of ["--healthy", "--warn", "--danger", "--accent"]) {
          const r = ratio(colour(vars, token), tint);
          if (r < 3) failures.push(`${theme} ${ground} dot ${token}: ${r.toFixed(2)}`);
        }
      }
    }
    expect(failures).toEqual([]);
  });
});

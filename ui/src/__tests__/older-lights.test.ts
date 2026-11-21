import { describe, expect, it } from "vitest";

import { block, colour, css, over, ratio } from "./cssTokens";

// A-25 Q10, the older lights: `flash` and the lit card take the one light's edgeless falloff. Light is
// spent before any box that holds it, so neither is a rectangle and neither is cut by its own card.
const braced = (from: string) => {
  const start = css.indexOf(from);
  expect(start, `${from} not in globals.css`).toBeGreaterThan(-1);
  let depth = 0;
  for (let i = css.indexOf("{", start); i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}" && --depth === 0) return css.slice(start, i + 1);
  }
  return "";
};
const flash = braced("@utility flash {");
const lit = braced("@utility lit-card {");
const keyframes = braced("@keyframes flash {");

const ramp = block(":root {\n    --violet-600");
const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };

describe("flash is a pool, not a rectangle", () => {
  it("fades the one light's strength, in its own box, and no longer a background colour", () => {
    expect(keyframes).not.toMatch(/background-color/);
    expect(keyframes).toMatch(/from \{\s*--flash-k: calc\(var\(--flash-reach\) \* var\(--hour-weight\)\);\s*\}\s*to \{\s*--flash-k: 0%;/);
    expect(css).toMatch(/@property --flash-k \{ syntax: "<percentage>"; inherits: false; initial-value: 0%; \}/);
    expect(flash).toMatch(/--pool-k: var\(--flash-k\);/);
    // closest-side in its own box: spent at the box's edge, drawn on the shared stops and nothing else.
    expect(flash).toMatch(/background-image: radial-gradient\(ellipse closest-side, var\(--pool-stops\)\);/);
    expect(flash).not.toMatch(/#[0-9a-f]{3,6}\b|rgb|transparent|accent-soft|var\(--accent\)/i);
    // The shared stops reach it (a default in the base layer).
    expect(css).toContain(":where(.hour-light, .lit-card, .flash, ");
  });

  it("keeps its meaning: once, 900ms on flash's curve, kept through reduced motion, quiet under forced colours", () => {
    expect(flash).toMatch(/animation: flash var\(--duration-flash\) var\(--ease-flash\) 1;/);
    expect(css).toMatch(/\.flash \{\s*animation-duration: var\(--duration-flash\) !important;/);
    expect(flash).toMatch(/@media \(forced-colors: active\) \{\s*background-image: none;\s*animation: none;/);
  });

  it("never takes text under 4.5:1 or a dot under 3:1 at its strongest, in both themes, under a quarter on paper", () => {
    const reach = Number(/--flash-reach:\s*(\d+)%/.exec(flash)?.[1]);
    expect(reach).toBeGreaterThan(0);
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const k = (reach * Number(vars["--hour-weight"])) / 100;
      if (theme === "light") expect(k * 100).toBeLessThan(25);
      for (const ground of ["--canvas", "--surface-1"]) {
        const litGround = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * k], colour(vars, ground));
        for (const token of ["--ink", "--muted", "--faint", "--accent-text", "--warn-text", "--danger-text"]) {
          const r = ratio(colour(vars, token), litGround);
          if (r < 4.5) failures.push(`${theme} ${ground} ${token}: ${r.toFixed(2)}`);
        }
        for (const token of ["--healthy", "--warn", "--danger", "--accent"]) {
          const r = ratio(colour(vars, token), litGround);
          if (r < 3) failures.push(`${theme} ${ground} dot ${token}: ${r.toFixed(2)}`);
        }
      }
    }
    expect(failures).toEqual([]);
  });
});

describe("the lit card's pool is spent before its card's edge", () => {
  const pool = /&::before \{([^}]*)\}/.exec(lit)?.[1] ?? "";

  it("is drawn closest-side in a box inside the card, no longer centred on its top edge", () => {
    expect(pool).toMatch(/radial-gradient\(ellipse closest-side, var\(--pool-stops\)\)/);
    expect(pool).not.toMatch(/ at \d+% 0|inset: 0;/);
    // Placed at the card's top corner, 70% wide, at most 120px tall, and never taller than the card.
    expect(pool).toMatch(/inset-block-start: 0;\s*inset-inline-start: 0;\s*width: 70%;\s*height: min\(100%, 120px\);/);
    // Behind the card's words (the card isolates), above its ground.
    expect(lit).toMatch(/isolation: isolate;/);
    expect(pool).toMatch(/z-index: -1;/);
  });

  it("keeps its colour, strength, still form and the forced-colours silence", () => {
    expect(lit).toMatch(/--lit-reach: 22%;/);
    expect(lit).toMatch(/--pool-k: calc\(var\(--lit-reach\) \* var\(--hour-weight\)\);/);
    expect(lit).not.toMatch(/animation|transition/);
    expect(lit).toMatch(/@media \(forced-colors: active\) \{\s*&::before,\s*&::after \{\s*display: none;/);
  });

  it("keeps a dot over 3:1 at its strongest, in both themes", () => {
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const k = (22 * Number(vars["--hour-weight"])) / 100;
      const ground = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * k], colour(vars, "--surface-1"));
      for (const token of ["--healthy", "--warn", "--danger", "--accent"]) {
        const r = ratio(colour(vars, token), ground);
        if (r < 3) failures.push(`${theme} ${token}: ${r.toFixed(2)}`);
      }
    }
    expect(failures).toEqual([]);
  });
});

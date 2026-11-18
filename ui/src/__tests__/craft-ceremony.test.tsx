// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

import { firstTime, idHash } from "@/lib/memory";

import { block, colour, css, over, ratio } from "./cssTokens";

/**
 * A-24 wave 4, ceremony (§3.2, §3.3, Q9): moments meant once are kept to once
 * by this device, and a device that can't remember shows nothing extra.
 */

const utility = (name: string) => {
  const start = css.indexOf(`@utility ${name} {`);
  let depth = 0;
  for (let i = css.indexOf("{", start); i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}" && --depth === 0) return css.slice(start, i + 1);
  }
  return "";
};
const keyframes = (name: string) => {
  const start = css.indexOf(`@keyframes ${name} {`);
  return start < 0 ? "" : css.slice(start, css.indexOf("\n  }\n", start) + 4);
};

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
});

describe("once, on this device", () => {
  it("is true the first time a mark is seen, then false, and keeps only the newest", () => {
    expect(firstTime("local", "k", "a", 2)).toBe(true);
    expect(firstTime("local", "k", "a", 2)).toBe(false);
    expect(firstTime("local", "k", "b", 2)).toBe(true);
    expect(firstTime("local", "k", "c", 2)).toBe(true);
    // "a" fell off the end: the store stays small on a shared computer.
    expect(JSON.parse(window.localStorage.getItem("k")!)).toEqual(["c", "b"]);
  });

  it("is false when the device can't keep it, so a moment meant once never repeats", () => {
    const full = { getItem: () => null, setItem: () => { throw new Error("full"); }, removeItem: () => {} } as unknown as Storage;
    vi.spyOn(window, "localStorage", "get").mockReturnValue(full);
    expect(firstTime("local", "k", "a", 2)).toBe(false);
    expect(firstTime("local", "k", "a", 2)).toBe(false);
  });

  it("keeps an id as a short hash, never the id", () => {
    expect(idHash("312987654321")).toMatch(/^[0-9a-f]{8}$/);
    expect(idHash("312987654321")).toBe(idHash("312987654321"));
    expect(idHash("312987654321")).not.toBe(idHash("312987654322"));
    expect(idHash("312987654321")).not.toContain("3129");
  });
});

describe("the door opens (§3.2): one keyframe, once, nothing travels", () => {
  const light = utility("hour-light");

  it("widens the arch and fades it, with no transform and no travel", () => {
    const open = keyframes("door-open");
    expect(open).toMatch(/from \{[^}]*--door-w: 90px;[^}]*--door-h: 260px;[^}]*--door-y: 80%;[^}]*opacity: 1;/);
    // It ends as the pool: the hour's own size and height, so the light widens INTO the pool.
    expect(open).toMatch(/to \{[^}]*--door-w: var\(--hour-w\);[^}]*--door-h: var\(--hour-h\);[^}]*--door-y: var\(--hour-y\);[^}]*opacity: 0;/);
    expect(open).not.toMatch(/transform|translate|scale|top:|left:|inset/);
    // The sizes are registered, so the arch widens rather than jumping halfway.
    for (const name of ["--door-w", "--door-h", "--door-y"]) {
      expect(css).toMatch(new RegExp(`@property ${name} \\{\\s*syntax: "<length-percentage>";`));
    }
  });

  it("plays the arch once on settle's curve and brings the pool up behind it", () => {
    const arch = /&\[data-arch\] \{[\s\S]*?\n  \}/.exec(light)?.[0] ?? "";
    // At the pool's own place, from the same tokens: never a fixed spot that crossfades.
    expect(arch).toMatch(/radial-gradient\(\s*ellipse var\(--door-w\) var\(--door-h\) at var\(--hour-x\) var\(--door-y\),/);
    expect(arch).toMatch(/animation: door-open var\(--duration-arrive\) var\(--ease-settle\) both;/);
    expect(light).toMatch(/radial-gradient\(\s*ellipse var\(--hour-w\) var\(--hour-h\) at var\(--hour-x\) var\(--hour-y\),/);
    expect(light).toMatch(/&\[data-door\] \{\s*animation: fade var\(--duration-arrive\) var\(--ease-settle\) both;/);
    expect(light).not.toMatch(/infinite/);
    // The doorway keeps the pool's mask, so it has no edge either: it never brings its own.
    expect(arch).not.toMatch(/mask|inset/);
  });

  it("is spent by the sign-in's first page, whichever it is: the shell notes every page", () => {
    // Home's own tests hold what happens after a note; this holds that the shell makes one.
    const shell = readFileSync(join(process.cwd(), "src/components/shell/AppShell.tsx"), "utf8");
    expect(shell).toMatch(/noteSignIn\(session\.signedInAt\)/);
  });

  it("is the pool at rest under reduced motion: no arch, and the pool not fading in", () => {
    const still = light.slice(light.indexOf("@media (prefers-reduced-motion: reduce)"));
    expect(still).toMatch(/&\[data-arch\] \{\s*display: none;/);
    expect(still).toMatch(/&\[data-door\] \{\s*animation: none;/);
  });
});

describe("the lit card (§3.3): the same light, under the same bounds", () => {
  const lit = utility("lit-card");

  it("is the accent and a hairline, still, clipped by its card, and gone under forced colours", () => {
    expect(lit).toContain("var(--accent)");
    expect(lit).not.toMatch(/--(healthy|warn|danger)\b|#[0-9a-f]{3,6}\b/i);
    expect(lit).not.toMatch(/animation|transition/);
    // Behind the card's words, above its ground.
    expect(lit).toMatch(/isolation: isolate;/);
    expect(lit).toMatch(/&::before \{[^}]*z-index: -1;/);
    expect(lit).toMatch(/@media \(forced-colors: active\)\s*\{\s*&::before,\s*&::after\s*\{\s*display: none;/);
  });

  it("never takes a card's text under 4.5:1 at its strongest, in both themes, and stays under a quarter on paper", () => {
    const reach = Number(/--lit-reach:\s*(\d+)%/.exec(lit)?.[1]);
    expect(reach).toBeGreaterThan(0);
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const weight = Number(vars["--hour-weight"]);
      if (theme === "light") expect(reach * weight).toBeLessThan(25);
      const litGround = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * ((reach * weight) / 100)], colour(vars, "--surface-1"));
      for (const token of ["--ink", "--muted", "--faint", "--accent-text"]) {
        const r = ratio(colour(vars, token), litGround);
        if (r < 4.5) failures.push(`${theme} ${token}: ${r.toFixed(2)}`);
      }
    }
    expect(failures).toEqual([]);
  });
});

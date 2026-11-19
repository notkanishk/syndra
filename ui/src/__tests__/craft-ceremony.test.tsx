// @vitest-environment jsdom
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

import { firstTime, idHash } from "@/lib/memory";

import { block, colour, css, over, ratio, SRC } from "./cssTokens";

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
const braced = (prelude: string) => {
  const start = css.indexOf(prelude);
  if (start < 0) return "";
  let depth = 0;
  for (let i = css.indexOf("{", start); i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}" && --depth === 0) return css.slice(start, i + 1);
  }
  return "";
};
const keyframes = (name: string) => braced(`@keyframes ${name} {`);
/** The opacity a keyframe set holds at `stop` (a stop list like "41%,\n  100%" counts). */
const opacityAt = (frames: string, stop: string) =>
  Number(new RegExp(`${stop}[^{]*\\{[^}]*opacity: ([\\d.]+);`).exec(frames)?.[1]);

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

describe("the door's light travels the greeting (§3.2): once, behind the words, nothing moves", () => {
  const band = braced("@utility greeting-light {");
  const travel = keyframes("greeting-light");
  const pool = keyframes("door-pool");
  const ms = (token: string) => Number(new RegExp(`${token}: (\\d+)ms;`).exec(css)?.[1]);

  it("is never a full-window layer: it lives in the greeting's box, and no door covers the page", () => {
    expect(css).not.toMatch(/\.door-through|--door-through|#app-root\[data-door\]/);
    expect(existsSync(resolve(SRC, "components/shell/DoorThrough.tsx"))).toBe(false);
    expect(band).toMatch(/position: absolute;/);
    expect(band).not.toMatch(/position: fixed|inset: 0;/);
    // The shell only notes the sign-in's first page; Home opens the door if it was that page.
    expect(readFileSync(resolve(SRC, "components/shell/AppShell.tsx"), "utf8")).toMatch(/noteSignIn\(session\.signedInAt\)/);
  });

  it("travels in reading direction, start of the words to their end, in one stroke", () => {
    // A position only at the first and last frames: one segment, so it never stops on the way.
    expect(travel.match(/background-position:/g)).toHaveLength(2);
    expect(travel).toMatch(/0% \{[^}]*background-position: 0% 50%;/);
    expect(travel).toMatch(/100% \{[^}]*background-position: 100% 50%;/);
  });

  it("changes only opacity and its gradient's position: nothing in the page moves", () => {
    for (const frames of [travel, pool]) {
      expect(frames).not.toBe("");
      expect(frames).not.toMatch(/transform|translate|scale|top:|left:|inset|margin|width|height/);
    }
  });

  it("is unhurried, on the settle curve, and hands over to the pool on the same stops", () => {
    const length = ms("--door-light");
    expect(length).toBeGreaterThanOrEqual(1100);
    expect(length).toBeLessThanOrEqual(1300);
    expect(band).toContain("animation: greeting-light var(--door-light) var(--ease-settle) both;");
    expect(css).toMatch(/&\[data-door\] \{\s*animation: door-pool var\(--door-light\) var\(--ease-settle\) both;/);
    // The band is whole until the stop where the pool starts, then each falls as the other rises.
    expect(opacityAt(travel, "65%")).toBe(1);
    expect(opacityAt(pool, "65%")).toBe(0);
    expect(opacityAt(travel, "100%")).toBe(0);
    expect(opacityAt(pool, "100%")).toBe(1);
  });

  it("is no band, and the pool at rest, under reduced motion or forced colours", () => {
    expect(band).toMatch(/@media \(prefers-reduced-motion: reduce\), \(forced-colors: active\) \{\s*display: none;/);
    expect(braced("@utility hour-light {")).toMatch(/@media \(prefers-reduced-motion: reduce\), \(forced-colors: active\) \{\s*&\[data-door\] \{\s*animation: none;/);
  });

  it("never takes a word under 4.5:1 at its brightest, in either room, and stays under a quarter on paper", () => {
    // Its brightest composite: the band alone at its heart (the crossfade keeps band and pool under the stronger
    // of the two, and the pool is held by craft-places).
    const reach = Number(/--door-light-reach: (\d+)%;/.exec(css)?.[1]);
    expect(reach).toBeGreaterThanOrEqual(25);
    expect(band).toContain("calc(var(--door-light-reach) * var(--hour-weight))");
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const weight = Number(vars["--hour-weight"]);
      if (theme === "light") expect(reach * weight).toBeLessThan(25);
      const lit = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * ((reach * weight) / 100)], colour(vars, "--canvas"));
      for (const token of ["--ink", "--muted", "--faint", "--accent-text"]) {
        const r = ratio(colour(vars, token), lit);
        if (r < 4.5) failures.push(`${theme} ${token}: ${r.toFixed(2)}`);
      }
    }
    expect(failures).toEqual([]);
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

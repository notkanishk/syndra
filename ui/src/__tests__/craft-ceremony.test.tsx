// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { firstTime, idHash } from "@/lib/memory";

import { block, colour, css, over, ratio, type RGBA } from "./cssTokens";

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

describe("walking through the door (§3.2): the door's own room, one keyframe set, nothing travels", () => {
  const sets = ["door-through-room", "door-through-arch", "door-through-light"].map(keyframes);
  const ms = (token: string) => Number(new RegExp(`${token}: (\\d+)ms;`).exec(css)?.[1]);

  it("changes only opacity and the light's size: no element travels", () => {
    for (const frames of sets) {
      expect(frames).not.toBe("");
      expect(frames).not.toMatch(/translate|top:|left:|inset|margin/);
      for (const [, transform] of frames.matchAll(/transform: ([^;]+);/g)) expect(transform).toMatch(/^scale\([\d.]+\)$/);
    }
  });

  it("keeps its stops on the tokens: the room dissolving over the tint before the reveal, 33% to 42%", () => {
    const through = ms("--door-through");
    const reveal = ms("--door-through-reveal");
    // Long enough to be registered as a door, not a flash (owner on staging, 19 Nov 2026).
    expect(reveal).toBeGreaterThanOrEqual(500);
    expect(Math.round((reveal / through) * 100)).toBe(42);
    expect(Math.round(((reveal - ms("--duration-tint")) / through) * 100)).toBe(33);
    // The page under it tints in at the reveal, and only then.
    expect(css).toMatch(/#app-root\[data-door\] \{\s*animation: fade var\(--duration-tint\) var\(--ease-tint\) var\(--door-through-reveal\) both;/);
  });

  it("is a moment, not a haze: the room whole and the arch and its light full until the reveal", () => {
    const [room, arch, light] = sets;
    expect(opacityAt(room, "0%")).toBe(1);
    expect(opacityAt(room, "33%")).toBe(1);
    expect(opacityAt(arch, "0%")).toBe(1);
    expect(opacityAt(light, "0%")).toBe(1);
    expect(opacityAt(light, "33%")).toBe(1);
    expect(Number(/--door-through-light: (\d+)%;/.exec(css)?.[1])).toBeGreaterThanOrEqual(50);
    // The room and the arch are gone at the reveal, before the page tints in: nothing is read through them.
    expect(opacityAt(room, "42%")).toBe(0);
    expect(opacityAt(arch, "42%")).toBe(0);
  });

  it("grows the arch and its light in one stroke on the settle curve: no hold, no pause at the reveal", () => {
    const [, arch, light] = sets;
    for (const frames of [arch, light]) {
      // A size only at the first and last frames: one segment, so the growth never stops on a stop.
      expect(frames.match(/transform:/g)).toHaveLength(2);
      expect(frames).toMatch(/0% \{[^}]*transform: scale/);
      expect(frames).toMatch(/100% \{[^}]*transform: scale/);
    }
    for (const part of ["room", "arch", "light"]) {
      expect(braced(`.door-through-${part} {`)).toContain(`animation: door-through-${part} var(--door-through) var(--ease-settle) both;`);
    }
  });

  it("is drawn from the door's own tokens, over the whole window, and never takes a click", () => {
    const layer = braced(".door-through {");
    expect(layer).toMatch(/position: fixed;\s*inset: 0;/);
    expect(layer).toContain("pointer-events: none;");
    expect(layer).toContain("overflow: hidden;");
    const drawn = [".door-through-room {", ".door-through-light {", ".door-through-arch {"].map(braced).join("\n");
    expect(drawn).not.toMatch(/#[0-9a-f]{3,6}\b|rgba?\(/i);
    for (const token of ["--door-page", "--door-fill", "--door-stroke", "--door-arch-w", "--door-arch-mask"]) expect(drawn).toContain(`var(${token})`);
  });

  it("is no layer, and no wait, under reduced motion or forced colours", () => {
    expect(css).toMatch(/@media \(prefers-reduced-motion: reduce\), \(forced-colors: active\) \{\s*\.door-through \{\s*display: none;\s*\}\s*#app-root\[data-door\] \{\s*animation: none;/);
  });

  it("never takes the page's text under 4.5:1 from the reveal on, in either room", () => {
    const light = sets[2];
    const heart = Number(/--door-through-light: (\d+)%;/.exec(css)?.[1]) / 100;
    // The strongest it is from the reveal on: the light's opacity only falls after that stop.
    const after = Math.max(opacityAt(light, "42%"), opacityAt(light, "100%"));
    const ramp = block(":root {\n    --violet-600");
    const door = block(":root {\n  /* Always-dark surface tokens.");
    const themes = { dark: { ...ramp, ...door, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...door, ...block('[data-theme="light"] {') } };
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const fill = colour(vars, "--door-fill");
      const veil: RGBA = [fill[0], fill[1], fill[2], heart * after];
      for (const ground of ["--canvas", "--surface-1", "--rail"]) {
        const under = colour(vars, ground);
        for (const token of ["--ink", "--muted", "--faint", "--accent-text"]) {
          const r = ratio(over(veil, over(colour(vars, token), under)), over(veil, under));
          if (r < 4.5) failures.push(`${theme} ${token} on ${ground}: ${r.toFixed(2)}`);
        }
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

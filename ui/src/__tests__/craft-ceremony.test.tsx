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

describe("the door's light glides into place on Home (§3.2): the hour's pool, once, and nothing else", () => {
  const glide = keyframes("door-open");
  const light = braced("@utility hour-light {");
  const stop = (at: string) => new RegExp(`(?:^|\\s)${at} \\{([^}]*)\\}`).exec(glide)?.[1] ?? "";

  it("travels from the head's left edge, a little smaller, to where the hour rests the pool", () => {
    expect(stop("from")).toMatch(/--door-x: -8cqw;\s*--door-s: 0\.8;\s*opacity: 0;/);
    expect(stop("25%")).toMatch(/^\s*opacity: 1;\s*$/);
    expect(stop("to")).toMatch(/--door-x: var\(--hour-x\);\s*--door-s: 1;/);
    // Registered, so the gradient glides rather than jumps; the pool's own size and height, scaled.
    expect(css).toMatch(/@property --door-x \{ syntax: "<length>";/);
    expect(css).toMatch(/@property --door-s \{ syntax: "<number>";/);
    expect(light).toMatch(/ellipse calc\(var\(--hour-w\) \* var\(--door-s\)\) calc\(var\(--hour-h\) \* var\(--door-s\)\) at var\(--door-x\) var\(--hour-y\),/);
  });

  it("is gentle and once: about 1.8s on the symmetric ease-in-out, light only, and never stronger than at rest", () => {
    expect(Number(/--door-open: (\d+)ms;/.exec(css)?.[1])).toBeGreaterThanOrEqual(1600);
    expect(light).toMatch(/&\[data-door\]::before \{[^}]*var\(--hour-light\)[^}]*animation: door-open var\(--door-open\) var\(--ease-tint\) both;/);
    expect(glide).not.toMatch(/transform|translate|scale|top:|left:|margin|inset|reach/);
  });

  it("is the pool at rest under reduced motion or forced colours", () => {
    expect(light).toMatch(/@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{\s*&\[data-door\]::before \{/);
    expect(light.replace(/\/\*[\s\S]*?\*\//g, "").replace(/@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{[\s\S]*?\n  \}\n/, "")).not.toContain("door-open");
  });

  it("touches nothing else: no staging of the head, no held blocks, no full-window layer", () => {
    expect(braced("@utility page-head {")).not.toContain("data-door");
    expect(css).not.toMatch(/--door-(blocks|lines|opened|wait|reach|through)\b|greeting-light|\.door-through/);
    expect(existsSync(resolve(SRC, "components/shell/DoorThrough.tsx"))).toBe(false);
    // The shell spends the door on the sign-in's first page, whichever it is; the server decides it.
    expect(readFileSync(resolve(SRC, "components/shell/AppShell.tsx"), "utf8")).toMatch(/spendDoor\(session\.signedInAt\)/);
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

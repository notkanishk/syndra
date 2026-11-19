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

describe("the door opens on Home (§3.2, board part 1 frames 5 to 7, at twice the board's time)", () => {
  const open = keyframes("door-open");
  const head = braced("@utility page-head {");
  const light = braced("@utility hour-light {");
  const ms = (token: string) => Number(new RegExp(`${token}: (\\d+)ms;`).exec(css)?.[1]);
  const stop = (at: string) => new RegExp(`(?:^|\\s)${at}[^{]*\\{([^}]*)\\}`).exec(open)?.[1] ?? "";

  // `--ease-settle` as a function of time: cubic-bezier(x1, y1, x2, y2), solved for x by bisection.
  const settle = (x: number) => {
    const [x1, y1, x2, y2] = (/--ease-settle: cubic-bezier\(([^)]+)\);/.exec(css)?.[1] ?? "").split(",").map(Number);
    const at = (p1: number, p2: number, t: number) => 3 * p1 * t * (1 - t) ** 2 + 3 * p2 * t ** 2 * (1 - t) + t ** 3;
    let [lo, hi] = [0, 1];
    for (let i = 0; i < 60; i += 1) {
      const mid = (lo + hi) / 2;
      if (at(x1, x2, mid) < x) lo = mid;
      else hi = mid;
    }
    return at(y1, y2, (lo + hi) / 2);
  };

  it("is the head itself, never a layer of its own: no band, no full-window door", () => {
    expect(css).not.toMatch(/\.door-through|--door-through|greeting-light|greeting-travel|--door-light\b/);
    expect(existsSync(resolve(SRC, "components/shell/DoorThrough.tsx"))).toBe(false);
    // The shell only notes the sign-in's first page; Home opens the door if it was that page.
    expect(readFileSync(resolve(SRC, "components/shell/AppShell.tsx"), "utf8")).toMatch(/noteSignIn\(session\.signedInAt\)/);
  });

  it("draws the board's arch, its opened pool and the hour's pool, in that order", () => {
    // Frame 5: the doorway's arch where the greeting will stand (the board's lights[0]).
    // Sizes are lengths in the light box's own units (cqw, cqh), so the board's px arch and % pool interpolate.
    // Brightest at its foot, which is the head's foot (the box ends 28px below it); never taller than the box
    // lets it show (to 70% of the ellipse, inside the top's fade), so nothing cuts it. Held to 15%: seen.
    expect(stop("0%")).toMatch(/--door-w: 90px;\s*--door-h: min\(260px, calc\(\(100cqh - 52px\) \/ 0\.7\)\);\s*--door-x: 70px;\s*--door-y: calc\(100cqh - 28px\);\s*--door-reach: 44%;/);
    expect(open).toMatch(/0%,\s*15% \{/);
    // Frame 6: opened into a pool (lights[1]).
    expect(stop("37%")).toMatch(/--door-w: 34cqw;\s*--door-h: 150cqh;\s*--door-x: 22cqw;\s*--door-y: -10cqh;\s*--door-reach: 32%;/);
    // Frame 7: resting where the hour puts it.
    expect(stop("100%")).toMatch(/--door-w: var\(--hour-w\);\s*--door-h: var\(--hour-h\);\s*--door-x: var\(--hour-x\);\s*--door-y: var\(--hour-y\);\s*--door-reach: var\(--hour-reach\);/);
    for (const name of ["--door-w", "--door-h", "--door-x", "--door-y"]) expect(css).toMatch(new RegExp(`@property ${name} \\{\\s*syntax: "<length>";`));
    expect(css).toMatch(/@property --door-reach \{\s*syntax: "<percentage>";/);
    expect(light).toContain("container-type: size;");
  });

  it("changes only light: no opacity, size or place of anything in the page", () => {
    expect(open).not.toMatch(/transform|translate|scale|opacity|top:|left:|margin|width:|height:/);
  });

  it("runs at twice the board's time, frame 6 at its stop, on the settle curve", () => {
    expect(ms("--door-open")).toBe(2 * 700);
    expect(ms("--door-opened")).toBe(2 * 260);
    expect(Math.round((ms("--door-opened") / ms("--door-open")) * 100)).toBe(37);
    expect(light).toMatch(/&\[data-door\]::before \{[^}]*animation: door-open var\(--door-open\) var\(--ease-settle\) both;/);
  });

  it("holds the blocks until the head has opened, on the door alone, never the head's settle clock", () => {
    // Frame 5 shows only the shell and the light: every block waits for --door-blocks, then arrives as on
    // every page (40ms apart, capped at five). Not gated on the head settling, a timer that runs at wall speed.
    expect(head).toMatch(/&\[data-door\] ~ \* \{\s*animation: arrive var\(--duration-arrive\) var\(--ease-arrive\) both;\s*animation-delay: calc\(var\(--door-blocks\) \+ \(var\(--arrive-cap\) - 1\) \* var\(--arrive-step\)\);/);
    expect(head).toMatch(/&\[data-door\] \+ \* \{\s*animation-delay: var\(--door-blocks\);/);
    for (let k = 1; k <= 3; k += 1) {
      const sel = `&\\[data-door\\]${" \\+ \\*".repeat(k + 1)} \\{\\s*animation-delay: calc\\(var\\(--door-blocks\\) \\+ ${k} \\* var\\(--arrive-step\\)\\);`;
      expect(head).toMatch(new RegExp(sel));
    }
    expect(head).not.toMatch(/&\[data-door\]:not\(\[data-settled\]\)|:not\(\[data-settled\]\)\[data-door\]/);
    // The door's rules come after the ordinary arrival's, so they win while it lasts.
    expect(head.indexOf("&[data-door] ~ *")).toBeGreaterThan(head.indexOf("&:not([data-settled]) ~ *"));
    expect(ms("--door-blocks")).toBeGreaterThanOrEqual(ms("--door-lines"));
  });

  it("holds boxes that paint: a list's display-contents wrapper holds still and its children wait", () => {
    // ArriveList's wrapper is `contents`: an opacity on it paints nothing, so its children would show at frame 5.
    const arrive = readFileSync(resolve(SRC, "lib/motion/arrive.tsx"), "utf8");
    expect(arrive).toMatch(/className = "contents"/);
    expect(head).toMatch(/&\[data-door\] ~ \.contents \{\s*animation: none;/);
    expect(head).toMatch(/&\[data-door\] ~ \.contents > \* \{\s*animation: arrive var\(--duration-arrive\) var\(--ease-arrive\) both;\s*animation-delay: calc\(var\(--door-blocks\) \+ \(var\(--arrive-cap\) - 1\) \* var\(--arrive-step\)\);/);
    expect(head).toMatch(/&\[data-door\] ~ \.contents > :nth-child\(1\) \{\s*animation-delay: var\(--door-blocks\);/);
    for (let k = 2; k <= 4; k += 1) {
      expect(head).toMatch(new RegExp(`&\\[data-door\\] ~ \\.contents > :nth-child\\(${k}\\) \\{\\s*animation-delay: calc\\(var\\(--door-blocks\\) \\+ ${k - 1} \\* var\\(--arrive-step\\)\\);`));
    }
    // After the list's own arrival rules, so the door's hold wins while it lasts.
    expect(css.indexOf("&[data-door] ~ .contents > *")).toBeGreaterThan(css.indexOf("@utility arrive-list {"));
  });

  it("keeps the head's full height from the first frame: its beats are opacity only, never removed", () => {
    for (const rule of head.match(/&\[data-door\] > [^{]+\{[^}]*\}/g) ?? []) {
      expect(rule).toMatch(/animation: (fade|none)/);
      expect(rule).not.toMatch(/display|visibility|height|max-height|content-visibility|position/);
    }
    // Home renders every beat from its first paint; nothing in the head waits on the door to exist.
    const home = readFileSync(resolve(SRC, "components/home/Home.tsx"), "utf8");
    expect(home).not.toMatch(/door\s*&&\s*</);
    for (const step of ["date", "lines"]) expect(home).toMatch(new RegExp(`^\\s*<div data-door-step="${step}">`, "m"));
  });

  it("holds only the light at first, then the greeting over it, the date line a beat behind, then the lines", () => {
    expect(head).toMatch(/&\[data-door\] \{\s*animation: none;/);
    expect(head).toMatch(/&\[data-door\] > h1 \{\s*animation: fade calc\(2 \* var\(--duration-tint\)\) var\(--ease-tint\)\s*calc\(var\(--door-opened\) - 2 \* var\(--duration-tint\)\) both;/);
    expect(head).toMatch(/&\[data-door\] > \[data-door-step="date"\] \{\s*animation: fade calc\(2 \* var\(--duration-tint\)\) var\(--ease-tint\)\s*calc\(var\(--door-opened\) - 2 \* var\(--duration-tint\) \+ 2 \* var\(--light-lead\)\) both;/);
    expect(head).toMatch(/&\[data-door\] > \[data-door-step="lines"\] \{\s*animation: fade calc\(2 \* var\(--duration-tint\)\) var\(--ease-tint\) var\(--door-lines\) both;/);
  });

  it("skips to frame 7 under reduced motion or forced colours", () => {
    // The door's light exists only where motion is welcome: otherwise the pool at rest, never the arch held still.
    expect(light).toMatch(/@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{\s*&\[data-door\]::before \{/);
    expect(light.replace(/\/\*[\s\S]*?\*\//g, "").replace(/@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{[\s\S]*?\n  \}\n/, "")).not.toContain("door-open");
    expect(head).toMatch(/@media \(prefers-reduced-motion: reduce\), \(forced-colors: active\) \{\s*&\[data-door\] > h1,\s*&\[data-door\] > \[data-door-step\],\s*&\[data-door\] ~ \*,\s*&\[data-door\] ~ \.contents > \* \{\s*animation: none;/);
  });

  it("never takes a word under 4.5:1: each beat tints in only where the light under it lets it", () => {
    const tint = ms("--duration-tint");
    const lead = ms("--light-lead");
    const [whole, opened] = [ms("--door-open"), ms("--door-opened")];
    const reaches = Array.from(light.matchAll(/--hour-reach:\s*(\d+)%/g)).map((m) => Number(m[1]));
    const pool = Math.max(...reaches);
    const number = (text: string, name: string) => Number(new RegExp(`${name}: ([\\d.]+)%`).exec(text)?.[1]);
    const [arch, frame6] = [number(stop("0%"), "--door-reach"), number(stop("37%"), "--door-reach")];
    // The light's strength at a moment: the arch held to its stop, then to frame 6, then to the hour's pool, each on settle.
    const held = (Number(/0%,\s*(\d+)% \{/.exec(open)?.[1]) / 100) * whole;
    const reachAt = (t: number) =>
      t <= held
        ? arch
        : t <= opened
          ? arch + (frame6 - arch) * settle((t - held) / (opened - held))
          : frame6 + (pool - frame6) * settle((t - opened) / (whole - opened));
    const beats: Array<[string, number, string[]]> = [
      ["the greeting", opened - 2 * tint, ["--ink", "--faint"]],
      ["the date line", opened - 2 * tint + 2 * lead, ["--muted", "--faint"]],
      ["the lines", ms("--door-lines"), ["--ink", "--muted", "--faint", "--accent-text"]],
    ];
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const failures: string[] = [];
    for (const [beat, at, tokens] of beats) {
      const reach = reachAt(at);
      for (const [theme, vars] of Object.entries(themes)) {
        const accent = colour(vars, "--accent");
        const weight = Number(vars["--hour-weight"]);
        const lit = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * ((reach * weight) / 100)], colour(vars, "--canvas"));
        for (const token of tokens) {
          const r = ratio(colour(vars, token), lit);
          if (r < 4.5) failures.push(`${beat} at ${at}ms (${reach.toFixed(1)}%), ${theme} ${token}: ${r.toFixed(2)}`);
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

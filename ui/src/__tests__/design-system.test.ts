import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Design-system canary.
 *
 * The palette, geometry and type roles live in globals.css and nowhere else.
 * This test fails when a file reaches around that: a leftover token from the
 * previous system, or a raw hex colour pasted from the design board instead of
 * the token that carries it in both themes.
 *
 * The previous version of this file guarded the OLD system's cleanup. That
 * system is gone; guarding its absence is what keeps it gone.
 */

const SRC_ROOT = resolve(__dirname, "..");
const GLOBALS_CSS = resolve(SRC_ROOT, "app/globals.css");

/** Utilities from the retired Obsidian Clarity / MD3 palette. */
const RETIRED_UTILITIES = [
  "bg-surface-container",
  "bg-surface-container-low",
  "bg-surface-container-high",
  "bg-surface-container-highest",
  "text-on-surface",
  "text-on-surface-variant",
  "text-on-background",
  "text-primary-container",
  "bg-primary-container",
  "text-on-primary",
  "border-outline",
  "border-outline-variant",
  "bg-error-container",
  "text-on-error-container",
  "glass-card",
  "bg-blob-hero",
  "font-display-fraunces",
];

const RETIRED_CSS_TOKENS = [
  "--color-surface-container",
  "--color-on-surface",
  "--color-primary-container",
  "--color-outline-variant",
  "--font-inter",
  "--font-fraunces",
];

/**
 * The design board's literal values. Every one of them has a token, and the
 * token is what keeps light theme correct — a hardcoded #7f5af0 stays the dark
 * room's violet on a white page, where the accent is a different one, and a
 * hardcoded #a3e635 stays a lime nobody can read on paper.
 */
const HARDCODED_COLOURS = [
  "#7f5af0",
  "#9b7bff",
  "#6f4ae0",
  "#a3e635",
  "#4d7c0f",
  "#d8f24e",
  "#f5a524",
  "#ff5c4d",
  "#141612",
  "#0b0c0a",
  "#101210",
  "#f3f5ef",
  "#7a5af8",
  "#1d1830",
];

const SKIP_DIRS = new Set(["__tests__", "node_modules", ".next", "dist"]);
const SKIP_FILES = new Set([resolve(__dirname, "design-system.test.ts")]);
const INCLUDED_EXTS = new Set([".ts", ".tsx"]);

function* walk(dir: string): Generator<string> {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      if (SKIP_DIRS.has(entry)) continue;
      yield* walk(path);
    } else if (stat.isFile()) {
      if (SKIP_FILES.has(path)) continue;
      const ext = entry.slice(entry.lastIndexOf("."));
      if (INCLUDED_EXTS.has(ext)) yield path;
    }
  }
}

function offenders(needles: string[]): Array<{ file: string; matches: string[] }> {
  const found: Array<{ file: string; matches: string[] }> = [];
  for (const file of walk(SRC_ROOT)) {
    const content = readFileSync(file, "utf8");
    const matches = needles.filter((needle) => content.includes(needle));
    if (matches.length > 0) found.push({ file: file.replace(SRC_ROOT, "src"), matches });
  }
  return found;
}

/** Same sweep, but pattern-matched — for shapes a substring can't express. */
function patternOffenders(pattern: RegExp, label: string): Array<{ file: string; matches: string[] }> {
  const found: Array<{ file: string; matches: string[] }> = [];
  for (const file of walk(SRC_ROOT)) {
    if (pattern.test(readFileSync(file, "utf8"))) {
      found.push({ file: file.replace(SRC_ROOT, "src"), matches: [label] });
    }
  }
  return found;
}

/** The brace-matched body of the named at-rule. Anchored so that a lookup for
 *  `@keyframes flash` cannot land on `@keyframes flash-value`. */
function namedAtRuleBody(css: string, keyword: string, name: string): string {
  const at = new RegExp(`${keyword}\\s+${name}\\s*\\{`).exec(css);
  return at ? atRuleBody(css.slice(at.index), `${keyword} ${name}`) : "";
}

/** The brace-matched body of the first at-rule whose prelude matches. */
function atRuleBody(css: string, prelude: string): string {
  const start = css.indexOf(prelude);
  if (start < 0) return "";
  const open = css.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}") {
      depth -= 1;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  return "";
}

function describeOffenders(list: Array<{ file: string; matches: string[] }>): string {
  return list.map((entry) => `  - ${entry.file}: ${entry.matches.join(", ")}`).join("\n");
}

describe("design system", () => {
  it("declares both themes and the three faces", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");

    // Dark is home, light is a full theme rather than a tint — both must
    // declare the whole palette, not a partial override.
    expect(css).toContain('[data-theme="dark"]');
    expect(css).toContain('[data-theme="light"]');

    for (const token of ["--accent", "--warn", "--danger", "--ink", "--surface-1", "--rail"]) {
      const declarations = css.split(`${token}:`).length - 1;
      expect(declarations, `${token} must be declared in both themes`).toBeGreaterThanOrEqual(2);
    }

    for (const font of ["--font-bricolage", "--font-figtree", "--font-jetbrains"]) {
      expect(css, `globals.css must wire ${font}`).toContain(font);
    }
  });

  // Three device states, named after devices. A component that writes
  // `tablet:` says which hand is holding the thing; one that writes `md:`
  // says 48rem, which is not where this shell breaks — the rail is 252px and
  // the widest fixed row is roughly 950px, so a stock `lg:` would collapse
  // columns a whole device early. Deleting these silently returns every
  // responsive class in the product to a scale nobody chose.
  it("names its breakpoints after devices, at the widths the design fixed", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");

    expect(css).toContain("--breakpoint-tablet: 45rem");
    expect(css).toContain("--breakpoint-desktop: 67.5rem");

    // Phone is the unprefixed base and must not acquire a token: giving it
    // one invites `phone:` classes, which is a desktop-first layout wearing
    // mobile-first names.
    expect(css, "phone is the base, not a variant").not.toContain("--breakpoint-phone");
  });

  // An action's result is reported by the surface that ran it: the row in the
  // row, a sheet as its own result, a plan on its result step. A toast breaks
  // that by construction — it reports every action in the same corner
  // regardless of where the operator is looking, covers the value they just
  // acted on, and removes the only account of what happened after four
  // seconds. It came out of the product in one pass; this is what stops it
  // arriving back one call site at a time.
  it("reports outcomes where they happened, never in a corner", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      const source = readFileSync(file, "utf8");
      if (
        /from ["']sonner["']/.test(source) ||
        /\btoast\.(success|error|warning|info)\(/.test(source)
      ) {
        found.push(file.replace(SRC_ROOT, "src"));
      }
    }
    expect(found, "use ActionOutcome — see lib/outcome.ts").toEqual([]);
  });

  it("keeps the retired palette out of globals.css", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of RETIRED_CSS_TOKENS) {
      expect(css, `globals.css must not declare ${token}`).not.toContain(token);
    }
  });

  it("no source file uses a retired palette utility", () => {
    const found = offenders(RETIRED_UTILITIES);
    expect(found, `Retired palette utilities still in source:\n${describeOffenders(found)}`).toEqual(
      [],
    );
  });

  it("no source file hardcodes a palette colour", () => {
    const found = offenders(HARDCODED_COLOURS);
    expect(
      found,
      `Hardcoded palette colours (use the token — light theme depends on it):\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // A component interpolated into a template literal renders as literal text:
  // `generated ${<Relative/>}` reaches the user as "generated $<Relative … />".
  // The stranded "$<" is the exact signature, and it survives type-checking.
  it("no component is interpolated into a string", () => {
    const found = offenders(["$<"]);
    expect(
      found,
      `A component inside a template literal renders as text:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // <a><button/></a> is invalid HTML and gives a keyboard or screen-reader user
  // two overlapping controls where the page shows one. Something that navigates
  // is a link: use ButtonLink, which shares the styling and nothing else.
  it("no button is nested inside a link", () => {
    const found = patternOffenders(/<Link[^>]*>\s*<Button[\s/>]/, "<Link><Button/></Link>");
    expect(
      found,
      `A button nested in a link — use ButtonLink:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // The doorway's breathing pool and animated grain are ambience: pleasant,
  // and nothing is lost without them. Motion nobody asked for is exactly the
  // motion a reduced-motion visitor must never be started on, so the guard is
  // opt-in — not "start it and let the global reduce rule rescue them".
  it("starts the doorway ambience only for a visitor who has not asked for less motion", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const guarded = atRuleBody(css, "@media (prefers-reduced-motion: no-preference)");
    expect(guarded, "globals.css must declare a no-preference guard").not.toBe("");

    for (const rule of ["animation: doorBreath", "animation: doorGrain"]) {
      expect(css.split(rule).length - 1, `${rule} must be declared exactly once`).toBe(1);
      expect(guarded, `${rule} must sit inside the no-preference guard`).toContain(rule);
    }
  });

  // ── Motion ──────────────────────────────────────────────────────────────

  // Six roles cover the product. A component that reaches past them for a raw
  // duration is a component whose timing nobody argued for, and the first one
  // to do it is how a design system starts being a suggestion.
  it("spends motion through the six named roles, never a raw duration", () => {
    const found = offenders(["transition-colors", "transition-all", "duration-1", "duration-2"]);
    expect(
      found,
      `Raw transition timings — use motion-tint / motion-press / motion-settle:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // "Only one thing loops" is the rule that makes a loop mean something. Two
  // states are licensed to say "this is still happening": degraded, and
  // loading. Everything else holds still, and the healthy state earns its calm
  // by being the only thing on screen that is perfectly still.
  it("licenses exactly one looping animation outside the doorway", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");

    // The doorway's ambience is its own, opt-in and guarded above.
    const app = css.replace(atRuleBody(css, "@media (prefers-reduced-motion: no-preference)"), "");
    const loops = app.match(/animation:[^;]*infinite[^;]*;/g) ?? [];

    expect(loops.length, `Looping animations found:\n${loops.join("\n")}`).toBeGreaterThan(0);
    for (const loop of loops) {
      expect(loop, `Only 'breathe' may loop, found: ${loop}`).toContain("breathe");
    }
  });

  // `arrive` staggers rows up to 200ms. Collapsing durations without also
  // collapsing delays leaves the sixth row of every list invisible for a fifth
  // of a second before it appears — a stall rather than an animation, which is
  // the exact thing the preference is asking us not to do.
  it("collapses delays as well as durations for reduced motion", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const reduce = atRuleBody(css, "@media (prefers-reduced-motion: reduce)");
    expect(reduce, "globals.css must declare a reduce block").not.toBe("");

    for (const property of [
      "animation-duration",
      "animation-delay",
      "animation-iteration-count",
      "transition-duration",
      "transition-delay",
    ]) {
      expect(reduce, `reduced motion must neutralise ${property}`).toContain(property);
    }
  });

  // A scrim is the whole viewport AND the parent of the dialog card, so a
  // transform on it does two wrong things at once: it drags a `fixed inset-0`
  // element off its own bottom edge, and it compounds with the card's rise so
  // the dialog travels further than the 10px anything is allowed to move.
  //
  // Pointing the scrim at a row-arrival keyframe is an easy mistake to make
  // and an invisible one to review — both animations are "fade in from
  // nothing" until you read the keyframes.
  it("fades the dialog scrim without moving it", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const scrim = atRuleBody(css, "@utility settle-scrim");
    expect(scrim, "globals.css must declare a settle-scrim utility").not.toBe("");

    const name = /animation:\s*([\w-]+)/.exec(scrim)?.[1];
    expect(name, "settle-scrim must name an animation").toBeTruthy();

    const frames = namedAtRuleBody(css, "@keyframes", name!);
    expect(frames, `@keyframes ${name} must exist`).not.toBe("");
    expect(frames, `a scrim only fades — @keyframes ${name} must not transform`).not.toContain(
      "transform",
    );
  });

  // The class only decides when the wash comes OFF; the animation itself is
  // authored in CSS. If the two drift, the row either loses its flash early or
  // sits in a finished animation doing nothing.
  it("times the flash class to the flash animation", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const declared = /--duration-flash:\s*(\d+)ms/.exec(css);
    expect(declared, "globals.css must declare --duration-flash").not.toBeNull();

    const hook = readFileSync(resolve(SRC_ROOT, "lib/useFlashOnChange.ts"), "utf8");
    const constant = /FLASH_MS = (\d+)/.exec(hook);
    expect(constant, "useFlashOnChange must export FLASH_MS").not.toBeNull();

    expect(constant![1], "FLASH_MS must equal --duration-flash").toBe(declared![1]);
  });

  // ── Colour roles ────────────────────────────────────────────────────────

  // --accent-ink on --accent is 4.18:1, which fails AA for anything smaller
  // than large text — and every filled control in this product carries a label
  // at 13.5px or below. --accent-dense takes the same label to 5.2:1. So the
  // rule is mechanical: a fill that carries text is dense, a fill that carries
  // nothing (a dot, a bar, a checkbox) keeps the brighter one.
  it("never puts a label on the bright accent fill", () => {
    const found = patternOffenders(
      /bg-accent(?![-\w])[^"'`]*text-accent-ink|text-accent-ink[^"'`]*bg-accent(?![-\w])/,
      "bg-accent + text-accent-ink",
    );
    expect(
      found,
      `Small text on the bright accent fails AA — use bg-accent-dense:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // Healthy means "nothing needed here": a dot, a word, or a hairline. There
  // is no healthy button and no healthy filled field, so the token set gives
  // you nothing to build one out of. The absence of these siblings IS the
  // enforcement — every other role has them.
  it("gives the healthy role no fill to be built out of", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(css, "globals.css must declare --healthy").toContain("--healthy:");
    for (const sibling of ["--healthy-soft", "--healthy-ink", "--healthy-line", "--healthy-text"]) {
      expect(css, `${sibling} would license a healthy fill — healthy is a dot, not a field`).not.toContain(
        sibling,
      );
    }
  });

  // A token that does not exist is the same failure as a hardcoded hex, and it
  // is quieter: `var(--fg-muted)` parses, the declaration is dropped, and the
  // element inherits — so a panel meant to carry an amber border and tint
  // renders as bare text and nobody sees an error. Three whole screens shipped
  // that way, painted in six names the design system had never heard of.
  //
  // The rule this enforces is the one globals.css already states: the palette
  // lives there and nowhere else. Reaching for a token is fine; inventing one
  // is not.
  it("uses no CSS variable globals.css does not declare", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const declared = new Set(
      Array.from(css.matchAll(/^\s*(--[a-z0-9-]+)\s*:/gm), (m) => m[1]),
    );

    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of walk(SRC_ROOT)) {
      const used = Array.from(
        readFileSync(file, "utf8").matchAll(/var\(\s*(--[a-z0-9-]+)/g),
        (m) => m[1],
      );
      const undeclared = [...new Set(used.filter((name) => !declared.has(name)))];
      if (undeclared.length > 0) {
        found.push({ file: file.replace(SRC_ROOT, "src"), matches: undeclared });
      }
    }

    expect(
      found,
      `CSS variables no theme declares — these render as nothing:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  it("focus is never removed", () => {
    // The accent focus ring is the keyboard user's only position indicator.
    const found = offenders(["outline-none", "focus:outline-0"]);
    expect(
      found,
      `Focus outline removed without a replacement:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });
});

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
 * token is what keeps light theme correct — a hardcoded #d8f24e stays lime on
 * a white page, where the accent is meant to be violet.
 */
const HARDCODED_COLOURS = [
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

  it("focus is never removed", () => {
    // The accent focus ring is the keyboard user's only position indicator.
    const found = offenders(["outline-none", "focus:outline-0"]);
    expect(
      found,
      `Focus outline removed without a replacement:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });
});

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

/**
 * Every opening `<tag …>` in a source file, brace-aware.
 *
 * A regex stopping at the first `>` is wrong for JSX: `onClick={() => …}`
 * closes the tag early and the className never gets read. That false negative
 * is why the touch-floor sweep below reported a clean repository while a 30px
 * destructive control sat in it.
 */
function openingTags(source: string, tag = "[A-Za-z][\\w.]*"): Array<{ at: number; text: string }> {
  const found: Array<{ at: number; text: string }> = [];
  const open = new RegExp(`<${tag}\\b`, "g");
  for (const match of Array.from(source.matchAll(open))) {
    const at = match.index;
    let depth = 0;
    for (let i = at; i < source.length; i += 1) {
      const char = source[i];
      if (char === "{") depth += 1;
      else if (char === "}") depth -= 1;
      else if (char === ">" && depth === 0) {
        found.push({ at, text: source.slice(at, i + 1) });
        break;
      }
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

  // `global-error.tsx` is the one file that cannot reach a token. It replaces
  // the document that loads globals.css, and it exists because the tree
  // holding the providers has just thrown — so importing anything from that
  // tree would re-enter what broke. Its exemption is paid for by the test
  // below, which holds it to importing nothing.
  const NO_TOKENS_AVAILABLE = "app/global-error.tsx";

  it("no source file hardcodes a palette colour", () => {
    const found = offenders(HARDCODED_COLOURS).filter(
      (entry) => !entry.file.endsWith(NO_TOKENS_AVAILABLE),
    );
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

  // "Never below 12.5px anywhere" — a legibility floor for somebody reading at
  // arm's length in a workshop, not a preference. `type-nav-group` sat at
  // 10.5px, which made the heading inside the mobile nav sheet the smallest
  // type in the product.
  it("keeps every named type role above the legibility floor", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const under: string[] = [];
    // Array.from rather than iterating the matcher: this file is type-checked
    // by a target that predates iterating an iterator directly, the same
    // reason the helpers above spell out their Sets and Maps.
    const roles = Array.from(
      css.matchAll(/@utility\s+(type-[\w-]+)\s*\{[^}]*?font-size:\s*([\d.]+)px/g),
    );
    for (const [, name, size] of roles) {
      if (Number(size) < 12.5) under.push(`${name} at ${size}px`);
    }
    expect(under, `Type roles below the 12.5px floor: ${under.join(", ")}`).toEqual([]);
  });

  // The tab bar carries `pb-[env(safe-area-inset-bottom)]`, so on a notched
  // phone it is up to 34px taller than its own content. A token that states a
  // bare pixel height puts anything positioned off it UNDER the bar — which is
  // exactly where the freshness dock landed, 2px inside it.
  it("measures the tab bar including the inset the device reserves", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const declared = /--touch-nav-height:\s*([^;]+);/g;
    const values = Array.from(css.matchAll(declared), (match) => match[1].trim());

    expect(values.length, "the token must be declared").toBeGreaterThan(0);
    const onPhone = values.filter((value) => value !== "0px");
    expect(onPhone.length, "the phone value must be declared").toBeGreaterThan(0);
    for (const value of onPhone) {
      expect(value, `--touch-nav-height: ${value} ignores the safe area`).toContain(
        "env(safe-area-inset-bottom)",
      );
    }
  });

  // A NUL written raw makes the file binary to `grep` and `file`, so the line
  // holding it drops out of every text sweep of the repository — including the
  // sweeps looking for bugs. `\u0000` is the same byte and stays greppable.
  it("no source file carries a raw NUL byte", () => {
    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (readFileSync(file, "utf8").includes("\u0000")) {
        found.push({ file: file.replace(SRC_ROOT, "src"), matches: ["raw NUL"] });
      }
    }
    expect(
      found,
      `Files invisible to grep:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // Every control that goes through `Button` clears the floor, and
  // touch-targets.test.tsx holds that contract. This is the other half: a raw
  // `<button>` skips `buttonClasses` entirely, which is how a 22px sheet
  // grabber and a 30px destructive kebab both shipped. It reads the literal
  // height off the tag rather than measuring anything, so it is a net rather
  // than a proof — but it is the net those two fell through.
  it("no raw button states a height below the touch floor", () => {
    const under: string[] = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (!file.endsWith(".tsx")) continue;
      const source = readFileSync(file, "utf8");
      for (const tag of openingTags(source, "button")) {
        const declared = /className=(?:"([^"]*)"|\{`([^`]*)`\})/.exec(tag.text);
        if (!declared) continue;
        const classes = declared[1] ?? declared[2] ?? "";
        const height = /(?:^|\s)(?:min-)?h-\[(\d+(?:\.\d+)?)px\]/.exec(classes);
        if (!height || Number(height[1]) >= 44) continue;
        // An explicit 44 anywhere in the string is the escape: a small ring
        // inside a full-size box is the fix, not a violation.
        if (/min-h-\[44px\]|min-h-11|(?:^|\s)h-11/.test(classes)) continue;
        const line = source.slice(0, tag.at).split("\n").length;
        under.push(`${file.replace(SRC_ROOT, "src")}:${line} h-[${height[1]}px]`);
      }
    }
    expect(under, `Controls under the 44px floor:\n  ${under.join("\n  ")}`).toEqual([]);
  });

  // The named roles above are only half the rule. A component can write
  // `text-[11.5px]` straight into a className and never touch globals.css,
  // which is where the other 14 breaches were living — the outcome pill among
  // them, on every result this product reports.
  //
  // The exemption is decoration, and it is checkable: every sub-floor size
  // left in the tree sits on an `aria-hidden` element — a bold "i" in a 20px
  // note badge, initials on a gradient the name is printed beside. Nothing
  // there is read, so a legibility floor for reading does not govern it.
  // Anything not inside a tag at all — a size map, a variant table — states
  // `type-floor-exempt` and says why.
  it("keeps raw type above the floor too, decoration excepted", () => {
    const SUB_FLOOR = /text-\[(\d+(?:\.\d+)?)px\]/g;
    const under: string[] = [];

    for (const file of Array.from(walk(SRC_ROOT))) {
      if (!file.endsWith(".tsx") && !file.endsWith(".ts")) continue;
      const source = readFileSync(file, "utf8");
      const tags = openingTags(source);

      for (const size of Array.from(source.matchAll(SUB_FLOOR))) {
        if (Number(size[1]) >= 12.5) continue;
        const at = size.index;

        // The tag this size is written into, if any: the last one that opens
        // before it and closes after it.
        const holder = tags.find((tag) => tag.at <= at && tag.at + tag.text.length > at);
        if (holder?.text.includes("aria-hidden")) continue;

        // A marker covers the eight lines under it — enough for a size map or
        // a variant table, short enough that it cannot silently cover a file.
        const line = source.slice(0, at).split("\n").length;
        const preceding = source.split("\n").slice(Math.max(0, line - 9), line).join("\n");
        if (preceding.includes("type-floor-exempt")) continue;

        under.push(`${file.replace(SRC_ROOT, "src")}:${line} ${size[0]}`);
      }
    }

    expect(under, `Readable type below the 12.5px floor:\n  ${under.join("\n  ")}`).toEqual([]);
  });

  it("keeps the last-resort boundary out of the tree it is catching", () => {
    const source = readFileSync(resolve(SRC_ROOT, NO_TOKENS_AVAILABLE), "utf8");
    const imports = Array.from(source.matchAll(/from\s+"([^"]+)"/g), (match) => match[1]);

    expect(
      imports.filter((from) => from.startsWith("@/") || from.startsWith(".")),
      "global-error.tsx may import nothing from the app: whatever threw may be a provider every one of those sits inside",
    ).toEqual([]);
  });

  // Two banners each sticking to `top-0` at the same z-index do not stack —
  // the later one paints over the earlier, which put the degraded banner on
  // top of the offline banner and inverted the ordering AppShell argues for.
  // One sticky slot holding both is the fix, so neither may reclaim its own.
  it("gives the two banners one sticky slot between them", () => {
    for (const name of ["states/OfflineBanner.tsx", "states/DegradedBanner.tsx"]) {
      const source = readFileSync(resolve(SRC_ROOT, "components", name), "utf8");
      expect(source, `${name} must not stick on its own`).not.toContain("sticky top-0");
    }
    const shell = readFileSync(resolve(SRC_ROOT, "components/shell/AppShell.tsx"), "utf8");
    const slot = /<div className="sticky top-0 z-40">\s*<OfflineBanner \/>\s*<DegradedBanner \/>/;
    expect(slot.test(shell), "AppShell must hold both banners in one sticky slot").toBe(true);
  });

  // iOS ignores manifest icons entirely and rasterises nothing, so without a
  // PNG the installed app — the whole reason members were given one — launches
  // with a blank tile.
  it("ships a raster touch icon, because iOS reads no other kind", () => {
    const icon = resolve(SRC_ROOT, "app/apple-icon.png");
    expect(statSync(icon).isFile(), "app/apple-icon.png must exist").toBe(true);
    // PNG magic, so a placeholder or a renamed SVG fails here rather than on a
    // home screen.
    expect(Array.from(readFileSync(icon).subarray(0, 4))).toEqual([0x89, 0x50, 0x4e, 0x47]);
  });
});

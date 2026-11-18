import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

import {
  ARRIVE_CAP,
  ARRIVE_MS,
  ARRIVE_SETTLED_MS,
  ARRIVE_STEP_MS,
  FOLD_MS,
  LIGHT_LEAD_MS,
} from "@/lib/motion";

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

/** A tag's own `className` (a string or a template), skipping any inside its props' braces. */
function topLevelClassName(tagText: string): string | undefined {
  let depth = 0;
  for (let i = 0; i < tagText.length; i += 1) {
    const char = tagText[i];
    if (char === "{") depth += 1;
    else if (char === "}") depth -= 1;
    else if (depth === 0 && tagText.startsWith("className=", i)) {
      const m = /^className=(?:"([^"]*)"|\{`([^`]*)`\})/.exec(tagText.slice(i));
      return m ? (m[1] ?? m[2]) : undefined;
    }
  }
  return undefined;
}

/** Source with its comments blanked, line numbers kept (a duration in a comment is prose). */
function withoutComments(source: string): string {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "))
    .replace(/(^|\s)\/\/.*$/gm, (_m, lead) => lead);
}

/** Numeric delays above 0 given to setTimeout/setInterval: `{ line, delay }`. */
function literalTimers(code: string): Array<{ line: number; delay: string }> {
  const found: Array<{ line: number; delay: string }> = [];
  for (const call of Array.from(code.matchAll(/\bset(?:Timeout|Interval)\(/g))) {
    let depth = 1;
    let comma = -1;
    let quote = "";
    let i = call.index + call[0].length;
    for (; i < code.length && depth > 0; i += 1) {
      const char = code[i];
      if (quote) {
        if (char === "\\") i += 1;
        else if (char === quote) quote = "";
      } else if (char === '"' || char === "'" || char === "`") quote = char;
      else if ("([{".includes(char)) depth += 1;
      else if (")]}".includes(char)) depth -= 1;
      else if (char === "," && depth === 1) comma = i;
    }
    const arg = code.slice(comma + 1, i - 1).trim();
    if (comma >= 0 && /^\d[\d_]*$/.test(arg) && Number(arg.replace(/_/g, "")) > 0) {
      found.push({ line: code.slice(0, call.index).split("\n").length, delay: arg });
    }
  }
  return found;
}

/** The sign-in door's choreography keeps its own timings (FINAL §4, Q8: recommended, pending owner). */
const isDoorFile = (file: string) => file.endsWith("/components/login/ceremony.ts");
/**
 * `// not-motion: why` (a debounce, a poll) or `// door-exempt: why` (the door's own timer), on
 * the line or the line above, says a number is not held to the motion tokens.
 */
const MARKER = /(?:not-motion|door-exempt):/;
const markedNotMotion = (lines: string[], line: number) =>
  MARKER.test(lines[line - 1] ?? "") || MARKER.test(lines[line - 2] ?? "");

/** The literal times a line of code spells: ms, WAAPI `duration:`/`delay:`, or seconds on a style. */
function literalTimeIn(text: string): string | null {
  if (/\b\d+(?:\.\d+)?ms\b/.test(text)) return "a literal duration";
  if (/\b(?:duration|delay)\s*:\s*\d/.test(text)) return "a literal duration or delay";
  if (/(?:transition|animation|style|duration|delay)/i.test(text) && /(?<![\w.-])\d*\.?\d+s\b/.test(text)) return "a literal duration in seconds";
  if (/cubic-bezier\(/.test(text)) return "a literal easing";
  return null;
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

  it("marks only a checkbox as some chosen, never a radio group with nothing picked", () => {
    // A radio group with no choice matches :indeterminate, so a bare
    // input:indeterminate rule paints every unchosen radio seat as chosen.
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(css).not.toMatch(/input:indeterminate/);
    expect(css).toMatch(/input\[type="checkbox"\]:indeterminate/);
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
      `Raw transition timings — use motion-tint / control / the unfold and indicator utilities:\n${describeOffenders(found)}`,
    ).toEqual([]);

    // The same rule for literals (D-1 M11, §7): a duration is a token or a
    // still.ts constant, an easing is var(--ease-*). A debounce or a poll that
    // is not motion says so with `// not-motion: why`; the door's own timers say `// door-exempt: why`.
    const literals: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (isDoorFile(file) || !/\/(components|app|lib)\//.test(file) || file.includes("/lib/motion/")) continue;
      const raw = readFileSync(file, "utf8");
      const code = withoutComments(raw);
      const rawLines = raw.split("\n");
      const matches: string[] = [];
      code.split("\n").forEach((text, index) => {
        if (markedNotMotion(rawLines, index + 1)) return;
        const literal = literalTimeIn(text);
        if (literal) matches.push(`line ${index + 1}: ${literal}`);
      });
      for (const { line, delay } of literalTimers(code)) {
        if (!markedNotMotion(rawLines, line)) matches.push(`line ${line}: timer of ${delay}`);
      }
      if (matches.length > 0) literals.push({ file: file.replace(SRC_ROOT, "src"), matches });
    }
    expect(
      literals,
      `Literal motion timings in components — use a token or a still.ts constant (or mark // not-motion: why):\n${describeOffenders(literals)}`,
    ).toEqual([]);
  });

  // The guard above must actually catch what it claims to.
  it("sees a literal duration, easing and timer, and honours the marker", () => {
    const code = withoutComments("// 200ms in prose\nsetTimeout(() => go(a, b), 260);\nsetTimeout(run, 0);\nsetTimeout(run, DELAY_MS);\n");
    expect(literalTimers(code)).toEqual([{ line: 2, delay: "260" }]);
    expect(withoutComments("const a = 1; // 40ms\n/* 40ms */ const b = 2;")).not.toMatch(/40ms/);
    expect(markedNotMotion(["// not-motion: debounce", "setTimeout(x, 250);"], 2)).toBe(true);
    expect(markedNotMotion(["", "setTimeout(x, 250);"], 2)).toBe(false);
    expect(markedNotMotion(["// door-exempt: the door", "setTimeout(x, 200);"], 2)).toBe(true);
    // The literals a regex for "Nms" alone would miss.
    expect(literalTimeIn("el.animate(f, { duration: 300 })")).not.toBeNull();
    expect(literalTimeIn("opts = { delay: 40 }")).not.toBeNull();
    expect(literalTimeIn('style={{ transition: "opacity 0.3s" }}')).not.toBeNull();
    expect(literalTimeIn('el.style.transition = "all 1s ease"')).not.toBeNull();
    expect(literalTimeIn('style={{ transitionDuration: "200ms" }}')).not.toBeNull();
    expect(literalTimeIn("easing: 'cubic-bezier(.2,.8,.2,1)'")).not.toBeNull();
    expect(literalTimeIn("const rows = 5; // after 5s")).toBeNull();
    expect(literalTimeIn("transition: `transform ${SETTLE_MS}ms var(--ease-settle)`")).toBeNull();
  });

  // D-1 §7, motion test 1: the tokens that name numbers already in use, their
  // JS mirrors, and the literals they replaced.
  it("names the fold, the light lead and the travels once, and mirrors them in JS", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of [
      "--duration-fold: 200ms",
      "--light-lead: 40ms",
      "--travel-arrive: 8px",
      "--travel-lift: 6px",
      "--travel-nudge: 4px",
    ]) {
      expect(css, `globals.css must declare ${token}`).toContain(token);
    }
    expect(FOLD_MS, "FOLD_MS must equal --duration-fold").toBe(Number(/--duration-fold:\s*(\d+)ms/.exec(css)?.[1]));
    expect(LIGHT_LEAD_MS, "LIGHT_LEAD_MS must equal --light-lead").toBe(Number(/--light-lead:\s*(\d+)ms/.exec(css)?.[1]));

    // Outside the token's own declaration, no 40ms and no fold calc remain.
    const code = withoutComments(css)
      .split("\n")
      .filter((line) => !/^\s*--[\w-]+:\s*40ms/.test(line))
      .join("\n");
    expect(code, "a 40ms literal is --light-lead").not.toMatch(/\b40ms\b/);
    expect(code, "the fold is --duration-fold, not a calc").not.toMatch(/animation:\s*fold\s+calc\(/);
    for (const px of ["translateY(8px)", "translateY(6px)", "translateY(4px)"]) {
      expect(code, `${px} is a --travel-* token`).not.toContain(px);
    }

    // Two utilities nothing used.
    expect(css).not.toContain("@utility motion-press");
    expect(css).not.toContain("@utility motion-settle");
    for (const file of Array.from(walk(SRC_ROOT))) {
      expect(readFileSync(file, "utf8"), file).not.toMatch(/(?<![\w-])motion-(press|settle)(?![\w-])/);
    }
  });

  // The doorway's literals are its own tokens, and ceremony.ts's easings mirror them.
  it("holds the doorway's easings to its --door-* tokens", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const numbers = (text: string) => (/cubic-bezier\(([^)]*)\)/.exec(text)?.[1] ?? "").split(",").map(Number);
    const ceremony = readFileSync(resolve(SRC_ROOT, "components/login/ceremony.ts"), "utf8");
    expect(numbers(/const EASE = "([^"]*)"/.exec(ceremony)![1])).toEqual(numbers(/--door-ease:\s*([^;]*);/.exec(css)![1]));
    expect(numbers(/const EXIT = "([^"]*)"/.exec(ceremony)![1])).toEqual(numbers(/--door-ease-exit:\s*([^;]*);/.exec(css)![1]));
    // No raw time on the door's rules: each is a token.
    const door = css.slice(css.indexOf("The doorway — /login"));
    const rules = withoutComments(door)
      .split("\n")
      .filter((line) => !/^\s*--door-[\w-]+:/.test(line))
      .join("\n");
    expect(rules).not.toMatch(/\d(\.\d+)?m?s\s+(ease|ease-out|ease-in-out|steps)/);
  });

  // Regions inside one wrapper take the page's stagger (D-1 M1), and the
  // wrapper itself holds still so it is not one slab.
  it("passes the page-head stagger through a marked wrapper", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const head = atRuleBody(css, "@utility page-head");
    expect(head).toMatch(/~ \[data-arrive-through\] \{\s*animation: none;/);
    for (let k = 1; k <= 4; k += 1) {
      expect(head).toContain(`> :nth-child(${k}) {\n    --arrive-k: ${k - 1};`);
    }
    for (let from = 0; from <= 3; from += 1) {
      expect(head).toContain(`${"+ * ".repeat(from)}+ [data-arrive-through] {\n    --arrive-from: ${from};`);
    }
    expect(head).toContain("--arrive-from) + max(var(--arrive-k) - var(--arrive-skip), 0), var(--arrive-cap) - 1)");
    // The section pill (a sticky h-0 first child) takes no slot, and does not arrive.
    expect(head).toMatch(/\[data-arrive-through\]:has\(> \.sticky:first-child\) \{\s*--arrive-skip: 1;/);
    expect(head).toMatch(/~ \[data-arrive-through\] > \.sticky:first-child \{\s*animation: none;/);
    // A wrapper inside the wrapper holds still, and its children stagger on from its own slot.
    expect(head).toMatch(/> \[data-arrive-through\] \{\s*animation: none;/);
    for (let k = 1; k <= 4; k += 1) {
      expect(head).toContain(`> [data-arrive-through] > :nth-child(${k}) {\n    --arrive-j: ${k - 1};`);
    }
    expect(head).toContain("max(var(--arrive-k) - var(--arrive-skip), 0) + var(--arrive-j)");
  });

  // Every animated part has a still form (WCAG 2.3.3): wherever script moves
  // something, the file asks stillMotion(), or the movement itself is a call to
  // a primitive that does; CSS is one block. A file that merely imports
  // Crossfade does not get an unguarded requestAnimationFrame.
  it("asks for stillness wherever script moves anything", () => {
    // Calls that consult stillMotion() themselves.
    const CONSULTS = /\b(?:stillMotion|glideTo|useExitPhase)\b/;
    // Not animation: restores a scroll position on Back (a jump); a one-time paint flag.
    const NOT_MOTION_FILES: Record<string, string> = {
      "lib/place.ts": "restores a scroll position on Back, which is a jump",
      "components/ui/Notice.tsx": "a flag that the first paint is over",
    };
    const MOVES = /\.animate\(|requestAnimationFrame|\.style\.(?:transition|animation)\b|[^-\w](?:transition|animation):/g;
    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      const rel = file.slice(SRC_ROOT.length + 1);
      if (rel.startsWith("lib/motion/") || rel in NOT_MOTION_FILES || isDoorFile(file)) continue;
      const raw = readFileSync(file, "utf8");
      const code = withoutComments(raw);
      const lines = raw.split("\n");
      const matches: string[] = [];
      for (const move of Array.from(code.matchAll(MOVES))) {
        const line = code.slice(0, move.index).split("\n").length;
        if (markedNotMotion(lines, line)) continue;
        // The file asks, or the callback this sits in calls something that does.
        if (/\bstillMotion\b/.test(code)) continue;
        if (CONSULTS.test(code.slice(move.index, move.index + 400))) continue;
        matches.push(`line ${line}: moves things, and nothing here asks stillMotion()`);
      }
      if (matches.length > 0) found.push({ file: `src/${rel}`, matches });
    }
    expect(found, `Animated parts with no still form:\n${describeOffenders(found)}`).toEqual([]);

    // The door reads the reader's setting itself and collapses its own timings.
    expect(readFileSync(resolve(SRC_ROOT, "components/login/ceremony.ts"), "utf8")).toContain("prefers-reduced-motion: reduce");

    const css = readFileSync(GLOBALS_CSS, "utf8");
    const reduce = atRuleBody(css, "@media (prefers-reduced-motion: reduce)");
    for (const rule of [
      "animation-duration: 0.01ms !important",
      "animation-iteration-count: 1 !important",
      "transition-duration: 0.01ms !important",
      "animation-delay: 0.01ms !important",
      "transition-delay: 0.01ms !important",
    ]) {
      expect(reduce, `the reduced-motion block must keep ${rule}`).toContain(rule);
    }
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

    const hook = readFileSync(resolve(SRC_ROOT, "lib/motion/still.ts"), "utf8");
    const constant = /FLASH_MS = (\d+)/.exec(hook);
    expect(constant, "still.ts must export FLASH_MS").not.toBeNull();

    expect(constant![1], "FLASH_MS must equal --duration-flash").toBe(declared![1]);
  });

  // A-19 P12: the scrim, the four layers and the touch floor are tokens.
  it("declares the scrim in both themes, and the layers and the tap floor once", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const dark = css.slice(css.indexOf('[data-theme="dark"] {'), css.indexOf('[data-theme="light"] {'));
    const light = css.slice(css.indexOf('[data-theme="light"] {'));
    expect(dark).toContain("--scrim: rgba(0, 0, 0, 0.55)");
    // The violet-black of --shadow-dialog, never pure black, at 0.35.
    expect(light).toContain("--scrim: rgba(60, 40, 120, 0.35)");
    for (const token of ["--layer-bar: 20", "--layer-pill: 30", "--layer-dialog: 50", "--layer-popover: 60", "--tap: 44px"]) {
      expect(css, `globals.css must declare ${token}`).toContain(token);
    }
    // A popover opened in a dialog shows over it.
    expect(css.indexOf("--layer-dialog")).toBeLessThan(css.indexOf("--layer-popover"));
  });

  // A-19 EOY-F11: the touch floor is --tap, spelled once.
  it("no component spells the touch floor as 44px or 11", () => {
    const found = patternOffenders(
      /(?:min-[hw]|\bh|\bsize)-(?:\[44px\]|11\b)|before:h-11/,
      "touch floor typed out",
    ).filter((entry) => !/\.(?:test|spec)\./.test(entry.file) && !entry.file.includes("__tests__"));
    expect(
      found,
      `Write the touch floor as min-h-[var(--tap)] (or size-[var(--tap)]):\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  it("no component writes a raw z-index number or a black overlay", () => {
    const found = patternOffenders(
      /\bz-(?:\[?\d|auto)|zIndex:\s*\d|z-index:\s*\d|\bbg-(?:black|white)\/\d|rgba\(\s*0\s*,\s*0\s*,\s*0/,
      "raw z-index or black overlay",
    ).filter((entry) => !entry.file.endsWith(NO_TOKENS_AVAILABLE));
    expect(
      found,
      `Use --layer-* for stacking and --scrim for overlays:\n${describeOffenders(found)}`,
    ).toEqual([]);
  });

  // A-05: the motion vocabulary is tokens, so a change to a curve or a cap is a
  // decision in one place. Each of these has a consumer in globals.css.
  it("declares the motion tokens Motion v2 names", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of [
      "--ease-arrive: cubic-bezier(0.16, 1, 0.3, 1)",
      "--ease-breathe: cubic-bezier(0.37, 0, 0.63, 1)",
      "--ease-flash: cubic-bezier(0.33, 1, 0.68, 1)",
      "--duration-glide: 520ms",
      "--travel-cap: 10px",
      "--arrive-step: 40ms",
      "--arrive-cap: 5",
    ]) {
      expect(css, `globals.css must declare ${token}`).toContain(token);
    }
  });

  // "Nothing in the page travels more than 10px." The phone sheet and a
  // control's own indicator are outside it and are not keyframes here.
  it("keeps every page keyframe inside the travel cap", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const cap = Number(/--travel-cap:\s*(\d+)px/.exec(css)?.[1]);
    expect(cap).toBe(10);
    for (const name of ["arrive", "settle-in", "popover-in", "lift", "flash-value"]) {
      const frames = namedAtRuleBody(css, "@keyframes", name);
      expect(frames, `@keyframes ${name} must exist`).not.toBe("");
      // A distance is a token (D-1 M11): resolve it, then hold it to the cap.
      for (const [, distance] of frames.matchAll(/translateY\(([^)]*\)?)\)/g)) {
        const named = /var\((--travel-[a-z]+)\)/.exec(distance)?.[1];
        const px = named ? Number(new RegExp(`${named}:\\s*(\\d+)px`).exec(css)?.[1]) : Number.parseFloat(distance);
        expect(px, `@keyframes ${name} travels ${distance}`).toBeLessThanOrEqual(cap);
      }
    }
    // The dialog's own rise, and a success's lift (S1, S6).
    expect(namedAtRuleBody(css, "@keyframes", "settle-in")).toContain("translateY(var(--travel-arrive)) scale(0.98)");
    expect(namedAtRuleBody(css, "@keyframes", "lift")).toContain("translateY(var(--travel-lift))");
  });

  // The arrive timers in TS are derived from the tokens, not repeated: a list
  // is settled when its last distinct delay plus one arrive has run, and the
  // sixth row onward waits no longer than the fifth (S11).
  it("derives the arrive timers from the tokens and caps the stagger at row five", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const token = (name: string) => Number(new RegExp(`${name}:\\s*(\\d+)`).exec(css)?.[1]);
    expect(ARRIVE_MS, "ARRIVE_MS must equal --duration-arrive").toBe(token("--duration-arrive"));
    expect(ARRIVE_STEP_MS, "ARRIVE_STEP_MS must equal --arrive-step").toBe(token("--arrive-step"));
    expect(ARRIVE_CAP, "ARRIVE_CAP must equal --arrive-cap").toBe(token("--arrive-cap"));
    expect(ARRIVE_SETTLED_MS).toBe(420 + 4 * 40);

    const list = atRuleBody(css, "@utility arrive-list");
    expect(list).toContain("animation-delay: calc((var(--arrive-cap) - 1) * var(--arrive-step))");
    expect(list).not.toContain("nth-child(6)");
  });

  // A row that leaves inside a settled list must still run its own exit: the
  // list's `animation: none` out-ranks `.close-gap` (the cascade, which jsdom
  // cannot see), and `LeavingRow` waits on that animation's end to remove it.
  it("never lets the list's own animation rules reach a leaving row", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const list = atRuleBody(css, "@utility arrive-list");
    const selectors = Array.from(list.matchAll(/([^{}]+)\{[^{}]*\banimation(?:-delay)?:/g)).map((m) => m[1].trim());
    const aimedAtEveryChild = selectors.filter((s) => !s.includes("[data-arriving]"));
    expect(aimedAtEveryChild.length).toBeGreaterThan(0);
    for (const selector of aimedAtEveryChild) {
      expect(selector, `${selector} must exclude .close-gap`).toContain(":not(.close-gap)");
    }
  });

  // S3: a phone sheet rises from its edge on settle; wider, it is the dialog's
  // own rise. Rows the reader asked for arrive on the list's own cap.
  it("rises a phone sheet from the bottom edge, and only below the tablet breakpoint", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(namedAtRuleBody(css, "@keyframes", "sheet-rise")).toContain("translateY(100%)");
    const sheet = atRuleBody(css, "@utility sheet-in");
    expect(sheet).toContain("animation: settle-in var(--duration-settle)");
    expect(atRuleBody(sheet, "@media (width < theme(--breakpoint-tablet))")).toContain(
      "animation: sheet-rise var(--duration-settle) var(--ease-settle) var(--light-lead) both",
    );
  });

  // Delays the answer removed: unfold's content waits one tint, not 140ms, and
  // nothing waits 280ms for a glide to end.
  it("waits one tint, not an ad-hoc delay, for an unfold's contents", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(css).not.toMatch(/\b(140|280)ms/);
    expect(atRuleBody(css, "@utility unfold")).toContain(
      "animation: fade var(--duration-settle) var(--ease-tint) var(--duration-tint) both",
    );
  });

  // Reduced motion keeps what answers the person or carries state, and drops
  // what is only movement (A-05 §2.8).
  it("lets press and flash survive reduced motion, drops the bloom, rests the breathing", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const reduce = atRuleBody(css, "@media (prefers-reduced-motion: reduce)");
    expect(reduce).toMatch(/\.press-scale\s*{[^}]*transition-duration: var\(--duration-press\)/);
    // A control keeps colour on tint and holds only the transform at press; the
    // seat's own press survives the same way.
    expect(reduce).toMatch(/\.control\s*{[^}]*var\(--duration-tint\)[^}]*var\(--duration-press\) !important/);
    expect(reduce).toMatch(/\.seat > span\s*{[^}]*var\(--duration-press\) !important/);
    expect(reduce).toMatch(/\.flash\s*{[^}]*animation-duration: var\(--duration-flash\)/);
    expect(reduce).toMatch(/\.bloom::after\s*{[^}]*animation: none/);
    expect(reduce).toMatch(/\.breathe\s*{[^}]*animation: none[^}]*opacity: 1/);
  });

  // Never: a loop other than breathe, a count that travels, a card that scales.
  it("keeps the skeleton still", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(atRuleBody(css, "@utility skeleton-bar")).not.toContain("animation");
  });

  it("scales only a sheet, never a card, a pill or a row", () => {
    const found = offenders(["settle-in"]).filter(
      (f) => !/(globals\.css|Modal\.tsx|TouchNav\.tsx|__tests__)/.test(f.file),
    );
    expect(found, `settle-in scales from 98% and is for sheets only:\n${describeOffenders(found)}`).toEqual([]);
  });

  it("moves no count: nothing but a chip uses flash-value", () => {
    const found = offenders(["flash-value"]).filter((f) => !/(globals\.css|__tests__)/.test(f.file));
    expect(found, describeOffenders(found)).toEqual([]);
  });

  // Hover is colour only; controls press, rows never scale.
  it("lets only a control scale when pressed", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    expect(atRuleBody(css, "@utility control")).toContain("scale(0.97)");
    const hover = atRuleBody(css, "@utility hover-tint");
    expect(hover).not.toContain("transform");
    // Pressed is one step beyond hover, in both themes, not a surface colour:
    // --tint-3 already is that step, so there is no token of its own (A-08 §1).
    expect(hover).toContain("var(--tint-3)");
    expect(hover).not.toContain("var(--surface-2)");
    expect(css).not.toContain("--hover-press");
    expect(css.match(/--tint-3:/g)).toHaveLength(2);
  });

  // The focus ring fades out on tint wherever a motion utility is present; the
  // utilities replace the base layer's transition, so each lists outline-color.
  it("lists outline-color in every motion utility, so a ring fades on tint", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const name of ["motion-tint", "control", "hover-tint"]) {
      expect(atRuleBody(css, `@utility ${name}`), `${name} must transition outline-color`).toContain("outline-color");
    }
    expect(atRuleBody(css, "@utility seat")).toContain("outline-color");
    // A hover:brightness fill tints too.
    expect(atRuleBody(css, "@utility motion-tint")).toContain("filter");
    expect(atRuleBody(css, "@utility control")).toContain("filter");
  });

  // Tailwind compiles rotate-90 to `rotate`, not `transform`: a chevron that
  // only lists transform snaps. Turning is settle's, the Unfold's own timing.
  it("turns a fold's chevron on settle, listing rotate", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const turn = atRuleBody(css, "@utility motion-turn");
    expect(turn).toContain("transition-property: rotate, transform");
    expect(turn).toContain("var(--duration-settle), var(--duration-settle)");
    expect(turn).toContain("var(--ease-settle)");
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
    // A role states px or reads a `--text-*` step (D-1); a step is checked at
    // every value it is declared with, the phone block's included.
    const steps = new Map<string, number[]>();
    for (const [, name, size] of Array.from(css.matchAll(/(--text-[a-z]+):\s*([\d.]+)px;/g))) {
      steps.set(name, [...(steps.get(name) ?? []), Number(size)]);
    }
    const roles = Array.from(
      css.matchAll(/@utility\s+(type-[\w-]+)\s*\{[^}]*?font-size:\s*(?:([\d.]+)px|var\((--text-[a-z]+)\))/g),
    );
    expect(roles.length, "the type roles must still be readable here").toBeGreaterThanOrEqual(8);
    for (const [, name, px, step] of roles) {
      const sizes = px ? [Number(px)] : (steps.get(step) ?? []);
      expect(sizes.length, `${name} reads ${step}, which globals.css does not declare in px`).toBeGreaterThan(0);
      for (const size of sizes) if (size < 12.5) under.push(`${name} at ${size}px`);
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
        if (/min-h-\[var\(--tap\)\]|(?:^|\s)h-\[var\(--tap\)\]/.test(classes)) continue;
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

  // jsdom computes no layout, so this cannot measure a scroll height — but
  // the whole failure was one missing class. A flex item defaults to
  // `min-height: auto` and refuses to shrink below its content, so `flex-1`
  // never constrained the scroller: it grew to the height of the page inside
  // it, `overflow-y-auto` had nothing to scroll, and the shell's own
  // `overflow-hidden` clipped the rest. On a phone that meant any route taller
  // than the viewport could not be scrolled at all and the tab bar sat below
  // the fold — on /roles at y=4037 of an 844px screen.
  it("lets the scroller shrink, or the phone cannot scroll at all", () => {
    const shell = readFileSync(resolve(SRC_ROOT, "components/shell/AppShell.tsx"), "utf8");
    const scroller = /<div id="app-scroll" className="([^"]+)"/.exec(shell);

    expect(scroller, "AppShell must still hold the scroller by that id").not.toBeNull();
    expect(scroller![1], "the scroller must be allowed to shrink below its content").toContain(
      "min-h-0",
    );

    // Its parent column is a flex item too, with the same default and the same
    // consequence. It is also the size container the page frame's `cqw` reads.
    expect(shell).toMatch(/<div className="@container flex min-h-0 min-w-0 flex-1 flex-col">/);
  });

  // Every dysfunction is a Notice in the content column (6a), never a band
  // under the top bar: the two system Notices open inside <main>, above the
  // page, offline first because it qualifies degraded.
  it("says the two system Notices inside the content column, offline first", () => {
    for (const name of ["states/OfflineBanner.tsx", "states/DegradedBanner.tsx"]) {
      const source = readFileSync(resolve(SRC_ROOT, "components", name), "utf8");
      expect(source, `${name} must not stick on its own`).not.toContain("sticky");
      expect(source, `${name} must use the Notice anatomy`).toContain("<Notice");
    }
    const shell = readFileSync(resolve(SRC_ROOT, "components/shell/AppShell.tsx"), "utf8");
    // A-20 P3, G7: under the page's header, in the seat ShellNotices holds.
    expect(
      /<main[^>]*>\s*<ShellNoticesProvider>\{children\}<\/ShellNoticesProvider>/.test(shell),
      "AppShell must wrap the page in the Notices' provider inside <main>, and mount no banner itself",
    ).toBe(true);
    expect(shell).not.toMatch(/<(Offline|Degraded)Banner/);
    const seat = readFileSync(resolve(SRC_ROOT, "components/shell/ShellNotices.tsx"), "utf8");
    expect(seat.indexOf("<OfflineBanner"), "offline above degraded").toBeLessThan(seat.indexOf("<DegradedBanner"));
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
  // Hover is colour only (FINAL §4): an underline cannot transition, and a
  // colour change with no transition class snaps. Both were on most name links.
  it("no hover underline, and no link changes colour on hover without a transition", () => {
    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (!file.endsWith(".tsx")) continue;
      const source = readFileSync(file, "utf8");
      const matches: string[] = [];
      if (/(?<![-\w])hover:underline/.test(source)) matches.push("hover:underline");
      for (const tag of openingTags(source, "(?:Link|a)")) {
        if (/hover:text-/.test(tag.text) && !/motion-tint|control|transition|link-hover/.test(tag.text)) {
          matches.push(`link line ${source.slice(0, tag.at).split("\n").length}: hover:text-* with no transition`);
        }
      }
      if (matches.length > 0) found.push({ file: file.replace(SRC_ROOT, "src"), matches });
    }
    expect(found, `Hover is colour only, on tint:\n${describeOffenders(found)}`).toEqual([]);
  });

  // A-08 §2a: every text link ends its hover in the same place, accent with its
  // underline tinted in, so a link's hover never depends on which link it is.
  // A brightness lift or an ink hover on a link is the old two-kinds-of-link.
  // A row that is a link (a nav item, a filled row) has a fill hover and stays.
  it("gives every text link `link-hover`, not its own hover colour or brightness", () => {
    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (!file.endsWith(".tsx")) continue;
      const source = readFileSync(file, "utf8");
      const matches: string[] = [];
      for (const tag of openingTags(source, "(?:Link|a)")) {
        const line = source.slice(0, tag.at).split("\n").length;
        if (/hover:brightness/.test(tag.text)) matches.push(`link line ${line}: hover:brightness`);
        if (/hover:text-(?:ink|accent-text)/.test(tag.text) && !/hover:bg-|hover-tint/.test(tag.text)) {
          matches.push(`link line ${line}: hover:text-ink / accent-text instead of link-hover`);
        }
      }
      if (matches.length > 0) found.push({ file: file.replace(SRC_ROOT, "src"), matches });
    }
    expect(found, `A link's hover is link-hover:\n${describeOffenders(found)}`).toEqual([]);
  });

  // A-17 G1: no control is text styled as a button. A <button> never wears a
  // link's hover, and the `link-tint` utility that dressed text buttons is gone.
  it("never styles a <button> as a link, and link-tint stays retired", () => {
    const found: Array<{ file: string; matches: string[] }> = [];
    for (const file of Array.from(walk(SRC_ROOT))) {
      if (!file.endsWith(".tsx")) continue;
      const source = readFileSync(file, "utf8");
      const matches = openingTags(source, "button")
        .filter((tag) => /link-hover|link-tint/.test(tag.text))
        .map((tag) => `button line ${source.slice(0, tag.at).split("\n").length}: use Button outline sm`);
      if (/\blink-tint\b/.test(source)) matches.push("link-tint is retired");
      if (matches.length > 0) found.push({ file: file.replace(SRC_ROOT, "src"), matches });
    }
    expect(found, `A button is Button:\n${describeOffenders(found)}`).toEqual([]);
    expect(readFileSync(GLOBALS_CSS, "utf8")).not.toContain("@utility link-tint");
  });

  it("draws link-hover's underline from transparent and keeps a resting underline", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const body = atRuleBody(css, "@utility link-hover");
    // The line is always on and only its colour moves: switching it on is a jump.
    expect(body).toMatch(/text-decoration-line:\s*underline/);
    expect(body).toMatch(/text-decoration-color:\s*transparent/);
    const hover = body.slice(body.indexOf("&:hover"));
    expect(hover).toContain("var(--accent-text)");
    expect(hover).toMatch(/text-decoration-color:\s*currentColor/);
    expect(hover).not.toContain("text-decoration-line");
    // On tint, ring included, like every motion utility.
    expect(body).toContain("var(--duration-tint)");
    expect(body).toContain("outline-color");
    // `link-hover underline`: a link underlined at rest keeps it.
    expect(body).toMatch(/&\.underline\s*\{[^}]*text-decoration-color:\s*currentColor/);
  });
});

// A-20 P14: one breakpoint for the phone shell, and the tap floor is a token.
describe("one breakpoint, one tap floor (A-20 P14)", () => {
  it("never switches layout at Tailwind's sm: (640px); tablet: (720px) is the one", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      const source = readFileSync(file, "utf8");
      // `sm:` as a variant on a utility, not a size named sm.
      for (const hit of source.matchAll(/(?<=[\s"'`])sm:[a-z[-]/g)) found.push(`${file}: ${hit[0]}`);
    }
    expect(found).toEqual([]);
  });

  it("writes the tap floor as var(--tap), never as 44, 48 or 52 pixels", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (file.endsWith("CopyableValue.tsx")) continue; // a read-only value's own row, not a tap target
      const source = readFileSync(file, "utf8");
      for (const hit of source.matchAll(/\b(?:w-11|min-h-\[(?:44|48|52)px\])(?![\w-])/g)) found.push(`${file}: ${hit[0]}`);
    }
    expect(found).toEqual([]);
  });
});

// D-1: the page read zoomed in at 100%. One density, set in @theme, read by the
// shared parts; these hold the numbers and the seams that carry them.
describe("density and alignment (D-1)", () => {
  const css = readFileSync(GLOBALS_CSS, "utf8");
  const theme = atRuleBody(css, "@theme {");
  const phone = atRuleBody(css.slice(css.indexOf("@layer base {")), "@media (max-width: 44.99rem) {");

  it("declares every density token at the value D-1 set, in @theme", () => {
    for (const token of [
      "--text-display: 34px",
      "--text-display--line-height: 1.1",
      "--text-page: 28px",
      "--text-page--line-height: 1.15",
      "--text-heading: 22px",
      "--text-heading--line-height: 1.2",
      "--text-title: 17px",
      "--text-title--line-height: 1.3",
      "--text-title--font-weight: 600",
      "--text-figure: 24px",
      "--text-body: 14px",
      "--text-body--line-height: 1.5",
      "--text-small: 13px",
      "--text-small--line-height: 1.45",
      "--text-fine: 12.5px",
      "--control-sm: 30px",
      "--control-md: 36px",
      "--page-max: 1280px",
      "--page-x: max(26px, calc((100cqw - var(--page-max)) / 2))",
      "--page-top: 24px",
      "--bar: 56px",
      "--measure: 60ch",
      "--field-max: 400px",
      "--region-gap: 40px",
      "--region-head-gap: 12px",
      "--card-x: 20px",
      "--card-head-y: 12px",
      "--row-y: 10px",
      "--row-gap: 16px",
      "--empty-y: 20px",
      "--dot-indent: 14px",
      "--seat-label: 170px",
      "--seat-number: 110px",
      "--seat-action-narrow: 140px",
      "--seat-action: 200px",
      "--seat-action-wide: 300px",
      "--seat-status: 240px",
    ]) {
      expect(theme, `@theme must declare ${token}`).toContain(`${token};`);
    }
    // The display steps' one step down, on a phone; nothing else moves there.
    expect(phone).toContain("--text-display: 28px;");
    expect(phone).toContain("--text-page: 26px;");
  });

  it("has each display role read its step, and none clamp with the window", () => {
    const reads: Record<string, string> = {
      "type-greeting": "--text-display",
      "type-page-title": "--text-page",
      "type-section-title": "--text-heading",
      "type-dialog-title": "--text-heading",
      "type-card-title": "--text-title",
      "type-empty-title": "--text-title",
    };
    for (const [role, step] of Object.entries(reads)) {
      const body = namedAtRuleBody(css, "@utility", role);
      expect(body, `${role} must read ${step}`).toContain(`font-size: var(${step});`);
      expect(body, `${role} must take ${step}'s line-height`).toContain(`line-height: var(${step}--line-height);`);
    }
    const clamped = Array.from(css.matchAll(/@utility\s+(type-[\w-]+)\s*\{[^}]*clamp\(/g), (m) => m[1]);
    expect(clamped, "a clamp saturates at 1080 and gives every desktop the largest size").toEqual([]);
  });

  it("gives SectionPill the regions' own gap, as the token, so the two agree at every width", () => {
    const overview = readFileSync(resolve(SRC_ROOT, "components/targets/TargetOverview.tsx"), "utf8");
    expect(overview).toContain('<div data-arrive-through className="grid gap-(--region-gap)">');
    expect(overview).toMatch(/<SectionPill[^>]*gap="var\(--region-gap\)"/);
  });

  // §8: "every phone layout is kept". The density is a window's; a phone reads
  // the same tokens at the values it had before D-1, and the parts scope what
  // the tokens cannot say (an inset, a wrapped gap) to tablet and up.
  it("keeps every phone layout: the phone holds each density token at its old value", () => {
    for (const token of [
      "--text-title: 19px", // card and empty titles, a Notice's
      "--region-gap: 32px",
      "--region-head-gap: 14px",
      "--card-head-y: 16px", // CardHeader's py-4
      "--row-y: 14px", // a row's py-3.5
      "--row-gap: 18px", // a row's gap-[18px], both ways
      "--empty-y: 32px", // EmptyState's py-8
    ]) {
      expect(phone, `the phone block must hold ${token}`).toContain(`${token};`);
    }
    const read = (path: string) => readFileSync(resolve(SRC_ROOT, path), "utf8");
    const parts: Array<[string, RegExp, string]> = [
      ["components/states/index.tsx", /px-6 py-\(--empty-y\) tablet:px-\(--card-x\)/, "EmptyState's 24px inset on a phone"],
      ["components/ui/Notice.tsx", /px-5 py-4 tablet:py-3\.5/, "a Notice's 16px on a phone"],
      ["components/ui/Card.tsx", /gap-\[11px\] px-\(--card-x\) py-\(--card-head-y\) tablet:gap-y-2/, "CardHeader's 11px wrap on a phone"],
      ["components/ui/Card.tsx", /gap-y-\(--row-gap\) tablet:gap-y-1/, "a wrapped part 18px under its row on a phone"],
      ["components/targets/Region.tsx", /grid gap-1\.5 tablet:gap-1/, "a region's 6px heading-to-lede on a phone"],
    ];
    for (const [path, shape, what] of parts) expect(read(path), what).toMatch(shape);
    // The row-top offset applies only where the seat shares the line.
    const rowTop = css.slice(css.indexOf(":where(.row-top)") - 200, css.lastIndexOf(":where(.row-top)") + 200);
    expect(rowTop).toContain("@media (width >= theme(--breakpoint-tablet))");
    expect(css).not.toMatch(/@utility row-top/);
  });

  // The headings sat 2px off their cells in about ten tables: CardColumns took
  // the token and the hand-built rows kept 18px. One grid, `row-cols`.
  it("gives every table row and its headings one column grid, and no column gap of their own", () => {
    const COLUMN_GAP = /(?:^|\s)(?:[a-z]+:)?gap-(?!y-)[^\s"'`]+/;
    // Bands and panels in a table's card that are not its rows: no columns to line up.
    const notRows: Array<[string, string]> = [
      ["app/changes/roles-that-wait/page.tsx", "gap-3 px-5 py-3.5"], // the waiting-changes footer
      ["components/system/ClaimsDelivery.tsx", "gap-x-3 gap-y-1.5 px-5 py-3.5"], // a part's head
      ["components/apps/DebugMode.tsx", "gap-x-3 gap-y-1 px-5 py-2.5"], // the log's head
    ];
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx")) continue;
      const rel = file.replace(`${SRC_ROOT}/`, "");
      const source = readFileSync(file, "utf8");
      // A CardRow's className never states a gap: the row has its grid.
      for (const tag of openingTags(source, "CardRow")) {
        // The row's own className, not one inside a prop's JSX (`action={<div className=…>}`).
        const cls = topLevelClassName(tag.text);
        if (cls && /(?:^|\s)(?:[a-z]+:)?gap-/.test(cls)) found.push(`${rel}: <CardRow className> states a gap`);
      }
      if (!source.includes("<CardColumns")) continue;
      for (const m of Array.from(source.matchAll(/(?:"|`)([^"`]*(?:row-divider|\$\{ROW\})[^"`]*)(?:"|`)/g))) {
        const cls = m[1];
        if (/bg-surface-0|(?:^|\s)grid(?:\s|$)/.test(cls)) continue; // a band or a grid panel
        if (notRows.some(([path, fragment]) => rel === path && cls.includes(fragment))) continue;
        const gap = COLUMN_GAP.exec(cls);
        if (gap) found.push(`${rel}:${source.slice(0, m.index).split("\n").length} ${gap[0].trim()}`);
      }
    }
    expect(found, `A table row takes row-cols (or ROW) and states only gap-y-*:\n  ${found.join("\n  ")}`).toEqual([]);
    expect(theme).toContain("--row-gap: 16px;");
    expect(namedAtRuleBody(css, "@utility", "row-cols")).toContain("column-gap: var(--row-gap);");
  });

  // §4: an action is never inside a sentence. The suite-wide DOM guard
  // (test-utils/layout.ts) reads what is rendered; this reads what is written,
  // so an untested screen is held too. A lede, a caption or a meta line may
  // carry a link (they are prose); nothing else may (pass 2 seated the rest).
  it("never writes an action after a sentence's space", () => {
    const allowed: Record<string, { count: number; why: string }> = {
      "app/roles/page.tsx": { count: 1, why: "the page's caption: a link in a caption is prose" },
      "components/apps/AppTokenScreen.tsx": { count: 2, why: "the header's meta line: a link in a meta line is prose" },
    };
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx")) continue;
      const rel = file.replace(`${SRC_ROOT}/`, "");
      const hits = Array.from(readFileSync(file, "utf8").matchAll(/\{" "\}\s*(?:\{[^<{}]*&&\s*)?<(?:Button|ButtonLink|ArrowLink)\b/g));
      if (hits.length !== (allowed[rel]?.count ?? 0)) found.push(`${rel}: ${hits.length}`);
    }
    expect(found, `An action belongs in its row's seat, not after a sentence:\n  ${found.join("\n  ")}`).toEqual([]);
  });

  // A detail whose parts all render nothing (Home's request note when nothing is held) must not
  // keep an empty line, and its gap, under the row. jsdom applies no CSS, so it is read here.
  // A seat holds verbs and a button's reason (up to 46ch). On a phone it wraps under its row,
  // and it must wrap inside itself too, never run past the card's edge (VersionBand's reason did).
  it("keeps a seat inside its row: it wraps, and is never wider than the row", () => {
    const seat = namedAtRuleBody(css, "@utility", "row-seat");
    expect(seat).toContain("flex-wrap: wrap;");
    expect(seat).toContain("max-width: 100%;");
    for (const width of ["--seat-action", "--seat-action-narrow", "--seat-action-wide"]) {
      expect(seat).toContain(`min-width: min(var(${width}), 100%);`);
    }
  });

  // Staging, 18 Nov, 1481px: a detail line and a caption were `flex-basis: 100%` capped by a
  // max-width of 60ch. Flex wraps an item by its basis clamped to its max-width, so on a wide card
  // the 60ch line fitted beside a short first line and sat there: the reports card's titles went a
  // word a line, and long captions sat at the right of their title. A line of its own is full width;
  // the measure is padding.
  it("gives a detail line and a caption a line of their own, the measure as padding, never a max-width", () => {
    const detail = namedAtRuleBody(css, "@utility", "row-detail");
    expect(detail).toContain("flex-basis: 100%;");
    expect(detail).toContain("padding-inline-end: max(0px, calc(100% - var(--measure)));");
    expect(detail).not.toMatch(/max-width/);
    expect(readFileSync(resolve(SRC_ROOT, "components/ui/Card.tsx"), "utf8")).toMatch(/<p data-prose data-caption className="row-detail">/);
    // And no screen builds the trap by hand: a full-line part capped by a measure.
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx")) continue;
      for (const m of Array.from(readFileSync(file, "utf8").matchAll(/(?:"|`)([^"`]*)(?:"|`)/g))) {
        const cls = ` ${m[1]} `;
        const fullLine = /\sbasis-full\s/.test(cls) || (/\sorder-last\s/.test(cls) && /\sw-full\s/.test(cls));
        if (fullLine && /\s(?:[a-z]+:)?(?:max-w-\S+|measure)\s/.test(cls)) found.push(`${file.replace(`${SRC_ROOT}/`, "")}: ${m[1]}`);
      }
    }
    expect(found, `A line of its own capped by a max-width can sit beside the line above; use CardRow's detail:\n  ${found.join("\n  ")}`).toEqual([]);
  });

  // Staging, 18 Nov: Home's "Gaps" stretched to 370px beside a long list, in a grid built by hand
  // without start alignment. Cards side by side go through CardGrid, which top-aligns them.
  it("puts cards side by side only through CardGrid, which top-aligns them", () => {
    const card = readFileSync(resolve(SRC_ROOT, "components/ui/Card.tsx"), "utf8");
    expect(card).toMatch(/data-card-grid className=\{`grid items-start gap-4 /);
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx") || file.endsWith("components/ui/Card.tsx")) continue;
      const source = readFileSync(file, "utf8");
      for (const m of Array.from(source.matchAll(/<div className="(grid [^"]*(?:grid-cols-2|auto-fit)[^"]*)">([\s\S]*?)<\/div>/g))) {
        // A grid whose cells are cards (a Card, or a part named …Card): CardGrid's job. A row of stat
        // tiles (StatCard) is a band of equal tiles, not cards of different heights, and stays its own.
        if (/<(?:Card|(?!StatCard)[A-Z]\w*Card)[\s>]/.test(m[2])) {
          found.push(`${file.replace(`${SRC_ROOT}/`, "")}:${source.slice(0, m.index).split("\n").length} ${m[1]}`);
        }
      }
    }
    expect(found, `Use CardGrid for cards side by side:\n  ${found.join("\n  ")}`).toEqual([]);
  });

  // §6 pass 4 (Q3, recommended, pending owner): six body sizes within 2.5px folded into three steps,
  // and the display sizes outside the scale onto it (§5 #40). A size is a step (`text-body`,
  // `text-small`, `text-fine`, `text-title`, `text-heading`, `text-figure`, or a `type-*` role),
  // never a pixel count: not a `text-[Npx]`, not an inline `fontSize` of its own, and not one of
  // Tailwind's own steps (`text-sm` and the rest are cleared with `--text-*: initial`, so they would
  // render nothing). Left as they are, each said where it is: decoration nobody reads (`aria-hidden`),
  // and a line marked `type-scale-exempt` on itself or on the line directly above (the avatar
  // initials, the dialog field, the wordmark, and global-error's written-out steps, which render
  // before any stylesheet could load).
  it("sets every size from the type scale, never a pixel count", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      const source = readFileSync(file, "utf8");
      const lines = source.split("\n");
      const tags = openingTags(source);
      const marked = (line: number, marker: string) => lines[line - 1].includes(marker) || (lines[line - 2] ?? "").includes(marker);
      const sizes = [
        ...Array.from(source.matchAll(/text-\[\d+(?:\.\d+)?px\]/g)),
        ...Array.from(source.matchAll(/fontSize\s*[:=]\s*\{?\s*["'`]?\d/g)),
        ...Array.from(source.matchAll(/(?<=[\s"'`:])text-(?:xs|sm|base|lg|xl|[2-9]xl)(?=[\s"'`])/g)),
      ];
      for (const size of sizes) {
        const at = size.index;
        const holder = tags.find((tag) => tag.at <= at && tag.at + tag.text.length > at);
        if (holder?.text.includes("aria-hidden")) continue;
        const line = source.slice(0, at).split("\n").length;
        if (marked(line, "type-scale-exempt")) continue;
        found.push(`${file.replace(`${SRC_ROOT}/`, "")}:${line} ${size[0]}`);
      }
    }
    expect(found, `Use a step of the type scale:\n  ${found.join("\n  ")}`).toEqual([]);
    // The body reads the body step, and Tailwind's own steps are cleared before the scale is declared.
    expect(atRuleBody(css.slice(css.indexOf("@layer base {")), "body {")).toContain("font-size: var(--text-body);");
    expect(theme.indexOf("--text-*: initial;")).toBeGreaterThan(-1);
    expect(theme.indexOf("--text-*: initial;")).toBeLessThan(theme.indexOf("--text-display: 34px;"));
  });

  // §3: "every sentence: lede, caption, Notice, guidance, a row's detail line" reads --measure, never
  // a cap of its own: no `max-w-[Nch]` anywhere, and no `max-w-[Npx]` on prose (a `<p>`, or a part
  // set in a text step, not a one-line label cut short). The one exemption is the greeting's balance widths (§6), marked
  // `measure-exempt` on its line or the line directly above.
  it("caps every sentence at the measure, never a ch or pixel count of its own", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx")) continue;
      const source = readFileSync(file, "utf8");
      const lines = source.split("\n");
      const rel = file.replace(`${SRC_ROOT}/`, "");
      const exempt = (at: number) => {
        const line = source.slice(0, at).split("\n").length;
        return lines[line - 1].includes("measure-exempt") || (lines[line - 2] ?? "").includes("measure-exempt");
      };
      for (const m of Array.from(source.matchAll(/(?<![\w-])max-w-\[\d+ch\]/g))) {
        if (!exempt(m.index)) found.push(`${rel}: ${m[0]}`);
      }
      for (const tag of openingTags(source)) {
        const cls = topLevelClassName(tag.text) ?? "";
        const px = /(?<![\w-])max-w-\[\d+px\]/.exec(cls);
        // A one-line label cut short (`truncate`, the account name in the top bar) is not a sentence.
        const prose = (/^<p\b/.test(tag.text) || /(?:^|\s)text-(?:body|small|fine)(?:\s|$)/.test(cls)) && !/(?:^|\s)truncate(?:\s|$)/.test(cls);
        if (px && prose && !exempt(tag.at)) found.push(`${rel}: ${px[0]} on prose`);
      }
    }
    expect(found, `Use measure:\n  ${found.join("\n  ")}`).toEqual([]);
  });

  // §6 pass 5: the canvas mounts the parts D-1 built, so the boards can drop their override block.
  // Every part Card.tsx exports is a design-system card with a preview, and a preview's own glue
  // reads the type scale, never a number of its own (BRIDGE, "The design system").
  it("puts every part of the row and the card on the canvas, its glue on the type scale", () => {
    const UI = resolve(SRC_ROOT, "..");
    const config = JSON.parse(readFileSync(resolve(UI, ".design-sync/config.json"), "utf8")) as { componentSrcMap: Record<string, string> };
    const exported = Array.from(
      readFileSync(resolve(SRC_ROOT, "components/ui/Card.tsx"), "utf8").matchAll(/^export function ([A-Z]\w*)/gm),
      (m) => m[1],
    );
    expect(exported).toEqual(expect.arrayContaining(["CardRow", "CardGrid", "ActionSeat", "CardColumns", "CardHeader"]));
    for (const name of exported) {
      expect(config.componentSrcMap[name], `${name} is a design-system card`).toBe("src/components/ui/Card.tsx");
      expect(readdirSync(resolve(UI, ".design-sync/previews")), `${name} has a preview`).toContain(`${name}.tsx`);
    }
    const literal: string[] = [];
    for (const file of readdirSync(resolve(UI, ".design-sync/previews"))) {
      const source = readFileSync(resolve(UI, ".design-sync/previews", file), "utf8");
      // A numeric inline size, or an arbitrary pixel text class, either way a size of its own.
      for (const m of Array.from(source.matchAll(/fontSize:\s*\d|text-\[\d+(?:\.\d+)?px\]/g))) {
        literal.push(`${file}:${source.slice(0, m.index).split("\n").length} ${m[0]}`);
      }
    }
    expect(literal, `A preview's glue reads a step (fontSize: "var(--text-body)", or text-body):\n  ${literal.join("\n  ")}`).toEqual([]);
    // The ActionSeats story itself shows the seat holding actions and the detail line: read within its
    // own block, so another story's `action=` cannot stand in for it.
    const row = readFileSync(resolve(UI, ".design-sync/previews/CardRow.tsx"), "utf8");
    const start = row.indexOf("export const ActionSeats");
    const story = row.slice(start, row.indexOf("export const", start + 1));
    expect(start, "CardRow's preview has its ActionSeats story").toBeGreaterThan(-1);
    expect(story).toMatch(/seat="default"\s+action=\{/);
    expect(story).toMatch(/detail=/);
    expect(readFileSync(resolve(UI, ".design-sync/previews/CardHeader.tsx"), "utf8")).toMatch(/caption=/);
    expect(readFileSync(resolve(UI, ".design-sync/previews/CardColumns.tsx"), "utf8")).toMatch(/seatLabel=/);
  });

  it("takes no line for an empty detail", () => {
    expect(namedAtRuleBody(css, "@utility", "row-detail")).toMatch(/&:empty \{\s*display: none;\s*\}/);
  });

  // §5 #29: seven hand-made seats at seven widths. A seat is ActionSeat, CardRow's
  // `seat` or CardColumns' `seat`, whose width is one of the tokens; a number's
  // seat is --seat-number. Neither is typed as pixels.
  it("types no seat of its own: no pixel width beside justify-end, no 110px number seat", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      if (!file.endsWith(".tsx") || file.endsWith("components/ui/Card.tsx")) continue;
      const rel = file.replace(`${SRC_ROOT}/`, "");
      for (const m of Array.from(readFileSync(file, "utf8").matchAll(/(?:"|`)([^"`]*)(?:"|`)/g))) {
        const cls = m[1];
        if (/(?:^|\s)(?:[a-z]+:)?(?:min-)?w-\[\d+px\]/.test(cls) && /(?:^|\s)justify-end(?:\s|$)/.test(cls)) found.push(`${rel}: ${cls}`);
        if (/(?:^|\s)(?:[a-z]+:)?w-\[110px\]/.test(cls) && /text-right/.test(cls)) found.push(`${rel}: ${cls}`);
      }
    }
    expect(found, `Use ActionSeat / seat, or w-(--seat-number):\n  ${found.join("\n  ")}`).toEqual([]);
  });

  it("lays the page frame once: main, the top bar's row and the section band read --page-x", () => {
    const read = (path: string) => readFileSync(resolve(SRC_ROOT, path), "utf8");
    expect(read("components/shell/AppShell.tsx")).toMatch(/<main[^>]*tablet:px-\(--page-x\) tablet:pt-\(--page-top\)/);
    expect(read("components/shell/TopBar.tsx")).toMatch(/<header[^>]*tablet:h-\(--bar\)[^>]*tablet:px-\(--page-x\)/);
    expect(read("components/ui/SectionNav.tsx")).toContain("tablet:-inset-x-(--page-x)");
    for (const path of ["components/shell/AppShell.tsx", "components/shell/TopBar.tsx", "components/ui/SectionNav.tsx"]) {
      expect(read(path), `${path} types the old inset`).not.toMatch(/(?:px|inset-x)-\[26px\]/);
    }
  });

  // §5 #37: sixteen list rows kept their 60px touch floor at every width.
  it("lifts every row's 60px floor where there is a pointer", () => {
    const found: string[] = [];
    for (const file of walk(SRC_ROOT)) {
      readFileSync(file, "utf8")
        .split("\n")
        .forEach((line, index) => {
          if (line.includes("min-h-[60px]") && !line.includes("tablet:min-h-0")) {
            found.push(`${file.replace(SRC_ROOT, "src")}:${index + 1}`);
          }
        });
    }
    expect(found, `Take ROW from components/ui/Card:\n  ${found.join("\n  ")}`).toEqual([]);
  });

  // §5 #30: the line under a status dot started at 14, 15, 16 or 18px. The
  // suite-wide DOM guard (test-utils/layout.ts) catches any `pl-*` on a line
  // under a rendered dot; this catches the two numbers the token replaced in
  // source no test renders.
  it("starts a line under a status dot at --dot-indent", () => {
    const found = patternOffenders(/\bpl-\[1[45]px\]/, "pl-[14px] or pl-[15px]: use pl-(--dot-indent)");
    expect(found, describeOffenders(found)).toEqual([]);
  });
});

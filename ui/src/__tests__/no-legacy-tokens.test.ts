import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC_ROOT = resolve(__dirname, "..");
const GLOBALS_CSS = resolve(SRC_ROOT, "app/globals.css");

const BANNED_UTILITIES = [
  // Legacy palette aliases (OCR-S1-05 compatibility block).
  "bg-surfaceHover",
  "bg-surface-hover",
  "hover:bg-surfaceHover",
  "hover:bg-surface-hover",
  "border-border",
  "text-foreground",
  "hover:text-foreground",
  "text-muted",
  "hover:text-muted",
  "bg-primary-hover",
  "hover:bg-primary-hover",
  "bg-primaryHover",
  "hover:bg-primaryHover",
  "bg-danger",
  "text-danger",
  "bg-danger-hover",
  // Hardcoded core-Tailwind tones that previously survived in JsonView
  // syntax highlighting and graph/page.tsx nodeTone(). Migrated to
  // success / warning / info semantic status tokens.
  "text-amber-500",
  "text-emerald-500",
  "text-sky-500",
  "text-amber-600",
  "text-emerald-600",
  "text-sky-600",
  "border-amber-500",
  "border-emerald-500",
  "border-sky-500",
  "bg-amber-500",
  "bg-emerald-500",
  "bg-sky-500",
  "dark:text-amber-300",
  "dark:text-emerald-300",
  "dark:text-sky-300",
  // Hardcoded zitadel/page.tsx flash + error tones. Migrated to
  // text-error / text-success. text-white is intentionally NOT banned
  // here — it's a neutral utility and may persist outside this change's
  // scope. In every chromatic-background pairing within this change's
  // touched files, text-white has been replaced with the matching
  // text-on-* token.
  "text-red-400",
  "text-emerald-400",
];

const BANNED_CSS_TOKENS = [
  "--color-foreground",
  "--color-surface-hover",
  "--color-border",
  "--color-primary-hover",
  "--color-muted",
  "--color-danger",
  "--color-danger-hover",
];

const SKIP_DIRS = new Set(["__tests__", "node_modules", ".next", "dist"]);
const SKIP_FILES = new Set([
  // The canary test itself enumerates the banned strings; exclude it.
  resolve(__dirname, "no-legacy-tokens.test.ts"),
]);
const INCLUDED_EXTS = new Set([".ts", ".tsx", ".css"]);

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

describe("no-legacy-tokens canary", () => {
  it("globals.css has no legacy compatibility token aliases", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of BANNED_CSS_TOKENS) {
      expect(css, `globals.css must not declare ${token}`).not.toContain(token);
    }
    expect(
      css,
      "globals.css must not carry the 'Legacy compatibility tokens' header",
    ).not.toMatch(/Legacy compatibility tokens/);
  });

  it("no source file uses a banned palette utility class", () => {
    const offenders: { file: string; matches: string[] }[] = [];
    for (const file of walk(SRC_ROOT)) {
      const content = readFileSync(file, "utf8");
      const found = BANNED_UTILITIES.filter((u) => content.includes(u));
      if (found.length > 0) {
        offenders.push({ file: file.replace(SRC_ROOT, "src"), matches: found });
      }
    }
    expect(
      offenders,
      `Banned palette utilities still in source:\n${offenders
        .map((o) => `  - ${o.file}: ${o.matches.join(", ")}`)
        .join("\n")}`,
    ).toEqual([]);
  });
});

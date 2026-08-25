import { readFileSync, existsSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * The merge model's vocabulary stops at the wire.
 *
 * Syndra borrowed a three-way merge for its targets, and the borrowing is
 * deliberate and narrow: three values, one verdict. What must NOT come with it
 * is the vocabulary. An operator deciding whether somebody bypassed access
 * control is not reading a rebase; "the target was changed by hand" is better
 * copy than "theirs-only conflict" for the person who has to choose, and the
 * moment the console starts speaking git it has made a specialist tool out of
 * a governance screen.
 *
 * Two guards. The first keeps git out of the copy. The second keeps the WIRE
 * codes out of it, cross-checked against the backend's own constants — a
 * resolution the backend can produce and the console cannot name renders the
 * identifier, which is exactly what `unbound` did.
 */

const UI_SRC = join(process.cwd(), "src");
const BACKEND = join(process.cwd(), "..", "backend");

function sourceFiles(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      if (entry === "__tests__" || entry === "node_modules") continue;
      sourceFiles(path, out);
      continue;
    }
    if (/\.(tsx|ts)$/.test(entry) && !/\.test\.tsx?$/.test(entry)) out.push(path);
  }
  return out;
}

/**
 * Comments are where the reasoning lives and are allowed to say "merge base"
 * as often as they need to. Only what can reach a screen is checked.
 */
function withoutComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(^|[^:])\/\/.*$/gm, "$1");
}

describe("the merge model's vocabulary stops at the wire", () => {
  it("keeps git out of anything that can reach a screen", () => {
    // Terms that only ever mean the version-control concept. `merge` and
    // `conflict` are deliberately absent: both are ordinary English here and
    // the console uses them correctly.
    const jargon = [
      /\brebase\b/i,
      /\bcherry[- ]pick/i,
      /\bfast[- ]forward/i,
      /\bthree[- ]way merge\b/i,
      /\bmerge base\b/i,
      /\bconflict marker/i,
      /\bours\/theirs\b/i,
      /\bgit\b/i,
      /HEAD~/,
    ];

    const offenders: string[] = [];
    for (const file of sourceFiles(UI_SRC)) {
      const body = withoutComments(readFileSync(file, "utf8"));
      for (const term of jargon) {
        if (term.test(body)) {
          offenders.push(`${file.replace(UI_SRC, "src")} — ${term}`);
        }
      }
    }
    expect(
      offenders,
      "Version-control vocabulary reached code that can render. The merge model " +
        "is an implementation borrowing, not a language the operator agreed to learn.",
    ).toEqual([]);
  });

  it("names every resolution the backend can produce", () => {
    const constants = join(BACKEND, "internal", "db", "merge_findings.go");
    if (!existsSync(constants)) {
      // Not skipped. A guard that quietly passes when it cannot find the thing
      // it checks is the failure this whole codebase keeps writing tests about.
      throw new Error(`cannot read ${constants}; this guard cannot be silently satisfied`);
    }
    const go = readFileSync(constants, "utf8");
    const declared = [...go.matchAll(/^\tResolution\w+\s*=\s*"([a-z_]+)"$/gm)].map((m) => m[1]);
    expect(declared.length, "no Resolution constants found — the guard has lost its subject")
      .toBeGreaterThan(0);

    const component = readFileSync(
      join(UI_SRC, "components", "targets", "MergeFindings.tsx"),
      "utf8",
    );
    const labelled = component.slice(component.indexOf("function decisionLabel"));
    const unnamed = declared.filter((code) => !labelled.includes(`case "${code}":`));

    expect(
      unnamed,
      "The backend can record these resolutions and the console has no words for them, " +
        "so it renders the identifier. `unbound` shipped that way.",
    ).toEqual([]);
  });

  it("never falls back to echoing the identifier", () => {
    const component = readFileSync(
      join(UI_SRC, "components", "targets", "MergeFindings.tsx"),
      "utf8",
    );
    const label = component.slice(
      component.indexOf("function decisionLabel"),
      component.indexOf("function show("),
    );
    // `default: return decision` is how a code reaches the page despite every
    // case above being correct — a deploy where the backend is ahead.
    expect(
      /default:\s*\n?\s*return decision;/.test(label),
      "The default must say something in English, not repeat the wire code.",
    ).toBe(false);
  });
});

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * A-20 P5 made one table of state words (`SYSTEM_WORDS`, TargetReading.tsx).
 * A table nobody is held to is a suggestion: this fails when a System file
 * writes one of those words as its own label instead of reading the table.
 * Sentences ("Syndra is waiting to retry TrueNAS", "it stopped answering")
 * are not state words and stay free; a word standing alone as a label, or
 * opening a string, is.
 */
const SRC = join(import.meta.dirname, "../../..");
const SCOPE = ["app/system", "app/zitadel", "components/targets", "components/storage", "components/system", "components/people/PersonStorage.tsx"];
const HOME = "components/targets/TargetReading.tsx";
const WORDS = /(^\s*|["'`>}]\s*)(Answering|Not answering|Offline|Unreachable|Unresponsive|Waiting to retry|Not connected)\b/;

function walk(path: string): string[] {
  if (statSync(path).isFile()) return [path];
  return readdirSync(path).flatMap((n) => (n === "__tests__" || n === "node_modules" ? [] : walk(join(path, n))));
}

const isComment = (line: string) => /^\s*(\/\/|\*|\/\*|\{\/\*)/.test(line);

export function hardCoded(source: string): string[] {
  return source.split("\n").filter((l) => !isComment(l) && WORDS.test(l));
}

describe("System state words come from SYSTEM_WORDS", () => {
  it("catches a hard-coded word and lets sentences and the table through", () => {
    expect(hardCoded(`<span>Offline</span>`)).toHaveLength(1);
    expect(hardCoded(`const w = "Not answering";`)).toHaveLength(1);
    expect(hardCoded("  Answering since noon")).toHaveLength(1);
    expect(hardCoded("title={`${name} isn't answering.`}")).toHaveLength(0);
    expect(hardCoded("<>{SYSTEM_WORDS.offline} since noon</>")).toHaveLength(0);
    expect(hardCoded("// Offline at 04:00")).toHaveLength(0);
  });

  it("no System file spells one out", () => {
    const files = SCOPE.flatMap((p) => walk(join(SRC, p))).filter(
      (f) => /\.tsx?$/.test(f) && !/\.test\./.test(f) && relative(SRC, f) !== HOME,
    );
    expect(files.length).toBeGreaterThan(10);
    const found = files.flatMap((f) => hardCoded(readFileSync(f, "utf8")).map((l) => `${relative(SRC, f)}: ${l.trim()}`));
    expect(found).toEqual([]);
  });
});

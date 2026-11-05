import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * A count of persons reads "people", never "users" or "members" (owner,
 * 5 Nov 2026). "Users" is the directory's word and "members" is a role's
 * (a role's members, new members), so a count is where the wrong one slips
 * in. This refuses a number, or a plural placeholder, standing next to either.
 */
const SRC = join(import.meta.dirname, "../../..");

const COUNTED = [
  /(\d|\})\s+(users?|members?)\b/i, // "12 users", "${n} members"
  /\b(users?|members?)\s*\(s\)/i, // "user(s)"
  /\b(number|count|total|how many)\s+of\s+(users|members)\b/i,
  /\b(title|label|noun|unit)\s*[=:]\s*\{?["'`](Users|Members)["'`]/, // a heading that carries a count
];

const isComment = (line: string) => /^\s*(\/\/|\*|\/\*|\{\/\*)/.test(line);
// Routes, query keys and the `.user` of a row are not copy.
const notCopy = (line: string) => /import |from "|\/users|queryKey|\.user\b|\buser:\s/.test(line);

export function countedAsUsers(source: string): string[] {
  return source.split("\n").filter((l) => !isComment(l) && !notCopy(l) && COUNTED.some((r) => r.test(l)));
}

function walk(path: string): string[] {
  if (statSync(path).isFile()) return [path];
  return readdirSync(path).flatMap((n) => (n === "__tests__" || n === "node_modules" ? [] : walk(join(path, n))));
}

describe("counts of persons read people", () => {
  it("catches a count that says users or members, and lets the rest through", () => {
    expect(countedAsUsers("<span>{n} users</span>")).toHaveLength(1);
    expect(countedAsUsers("`${rows.length} members hold it`")).toHaveLength(1);
    expect(countedAsUsers('"12 users"')).toHaveLength(1);
    expect(countedAsUsers('<CardHeader title="Members" count={n} />')).toHaveLength(1);
    expect(countedAsUsers("`${people(3)} should have it`")).toHaveLength(0);
    expect(countedAsUsers("Given to new members")).toHaveLength(0);
    expect(countedAsUsers("// 0 members alone reads as safe")).toHaveLength(0);
  });

  it("no UI copy counts people as users or members", () => {
    const files = ["app", "components", "lib"]
      .flatMap((p) => walk(join(SRC, p)))
      .filter((f) => /\.tsx?$/.test(f) && !/\.test\./.test(f));
    const found = files.flatMap((f) => countedAsUsers(readFileSync(f, "utf8")).map((l) => `${relative(SRC, f)}: ${l.trim()}`));
    expect(found).toEqual([]);
  });
});

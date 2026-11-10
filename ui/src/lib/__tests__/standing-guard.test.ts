import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * Holding is the backend's one answer (RoleStanding, one-path W3b): a screen
 * renders `standings` / `standing` with lib/standing.ts and never works out
 * whether a person holds a role from anything else. `in_zitadel` is gone from
 * the API; `observed_role_keys` is a LIST of what Zitadel shows, not a verdict.
 */
const SRC = join(__dirname, "..", "..");

// The only places that may name observed_role_keys, and why. Each uses it to
// LIST the roles Syndra never gave; what those roles say comes from `standings`.
const MAY_LIST_OBSERVED = new Set([
  "components/people/PersonAccess.tsx", // unexplainedIn: observed minus effective
  "components/member/MemberAccess.tsx", // the unexplained rows under a project's roles
]);

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) {
      if (entry !== "node_modules" && entry !== "__tests__") walk(p, out);
    } else if (/\.tsx?$/.test(p) && !/\.test\.tsx?$/.test(p)) out.push(p);
  }
  return out;
}

const files = [...walk(join(SRC, "components")), ...walk(join(SRC, "app"))];
const hits = (re: RegExp, allowed = new Set<string>()) =>
  files
    .map((f) => relative(SRC, f))
    .filter((f) => !allowed.has(f) && re.test(readFileSync(join(SRC, f), "utf8")));

describe("holding comes from the backend's standing", () => {
  it("no component reads in_zitadel", () => {
    expect(hits(/(?<![A-Za-z_])in_zitadel\b/)).toEqual([]);
  });
  it("only the unexplained lists read observed_role_keys", () => {
    expect(hits(/\bobserved_role_keys\b/, MAY_LIST_OBSERVED)).toEqual([]);
  });
  it("nothing decides holding from whether every source is still queued", () => {
    expect(hits(/\bnothingSentYet\b/)).toEqual([]);
  });
});

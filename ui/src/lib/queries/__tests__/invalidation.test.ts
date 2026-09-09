import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * `["propagation"]` (singular) matches nothing — the real key is
 * `["propagations", "pending"]` (see `usePropagation.ts`). Three mutations
 * shipped with exactly this typo, each believing it was refreshing the
 * pending-changes count while actually invalidating a key no query ever
 * used. A source scan is the only guard that would have caught it: the typo
 * type-checks and the mutation still succeeds, so nothing at runtime says
 * the cache went stale.
 */
const QUERIES_DIR = join(import.meta.dirname, "..");

function queryFiles(): string[] {
  return readdirSync(QUERIES_DIR)
    .filter((name) => /\.tsx?$/.test(name))
    .map((name) => join(QUERIES_DIR, name));
}

describe("invalidateQueries key hygiene", () => {
  it("never invalidates the singular, non-existent [\"propagation\"] key", () => {
    const offenders: string[] = [];
    // Matches "propagation" as an array element (followed by a comma or the
    // closing bracket) so it does not also flag the real "propagations" key.
    const singularKey = /\[\s*"propagation"\s*(,|\])/;

    for (const file of queryFiles()) {
      const src = readFileSync(file, "utf8");
      if (singularKey.test(src)) offenders.push(file);
    }

    expect(offenders).toEqual([]);
  });
});

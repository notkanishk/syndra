import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * A box that reports a fault is a Notice, and nothing draws its own "!" or "i"
 * badge (A-04 §7.1, 7.1a). The hand-copied badge was a 20px circle in a tone,
 * holding one letter, set beside a tinted note: four copies, no two alike. An
 * explanation is a caption or a lede instead, with no badge and no box.
 */
const SRC = join(import.meta.dirname, "../../..");

function* sources(dir: string): Generator<string> {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      if (name !== "__tests__" && name !== "node_modules") yield* sources(path);
    } else if (/\.tsx$/.test(name)) yield path;
  }
}

describe("callouts", () => {
  it("never hand-draw the badge a Notice replaced", () => {
    const badge = /h-5 w-5 flex-none items-center justify-center rounded-pill bg-(warn|danger|accent)-soft/;
    const found = [...sources(SRC)].filter((file) => badge.test(readFileSync(file, "utf8")));
    expect(found.map((f) => f.slice(SRC.length + 1))).toEqual([]);
  });
});

import { describe, expect, it } from "vitest";

import type { TargetReach } from "@/lib/queries/useTargets";

/**
 * A read always carries its date and a read not taken carries none, as `last_known` and
 * `MappableValues` do. The check is the type itself: `bun run typecheck` fails if either line
 * stops being an error.
 */
describe("what each group opens, as typed", () => {
  it("dates a read and only a read", () => {
    // @ts-expect-error a read without its date
    const undated: TargetReach = { read: true };
    // @ts-expect-error a read not taken, with a date
    const dated: TargetReach = { read: false, read_at: "2026-11-21T10:00:00Z" };
    const read: TargetReach = { read: true, read_at: "2026-11-21T10:00:00Z" };
    expect([undated, dated, read].map((r) => r.read)).toEqual([true, false, true]);
  });
});

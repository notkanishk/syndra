import { describe, expect, it } from "vitest";

import type { Read } from "@/lib/queries/read";
import type { TargetReach } from "@/lib/queries/useTargets";

/**
 * Design §13.6: a count, a list or a date exists only under `data`, which a read with nothing to
 * give does not have. The check is the type itself: `bun run typecheck` fails if either line stops
 * being an error.
 */
describe("what each group opens, as a Read", () => {
  it("offers no data on a read that has none, and dates a reading", () => {
    const none: Read<TargetReach> = { state: "none", why: { reason: "addon_unreachable" } };
    // @ts-expect-error a read with nothing to give has no groups to draw
    const groups = none.data;
    // @ts-expect-error a reading without its date
    const undated: Read<TargetReach> = { state: "now", data: { read: { state: "now" } } };
    expect([groups, undated.state]).toEqual([undefined, "now"]);
  });
});

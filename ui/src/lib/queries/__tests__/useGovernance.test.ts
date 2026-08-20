import { describe, expect, it } from "vitest";

import { mapGovernanceSummary, type GovernanceSummary } from "@/lib/queries/useGovernance";

/**
 * The summary mapper rebuilds its object field by field rather than spreading
 * it, which is right — a shape off the network is not a shape this app can
 * trust. The cost is a failure mode nothing else catches: a field added to the
 * type and read by a screen arrives as `undefined` until it is named in the
 * mapper. TypeScript sees the declared type; the screen sees a silent zero.
 *
 * `merge_findings` spent its whole life in that gap. It was declared, it was
 * inside Today's headline arithmetic, and it was dropped in transit, so the
 * count had never once been non-zero.
 */
describe("the governance summary survives its own mapper", () => {
  const wire: GovernanceSummary = {
    pending_requests: [{ id: "r1" }] as GovernanceSummary["pending_requests"],
    expiring_grants: [{ id: "g1" }] as GovernanceSummary["expiring_grants"],
    cleanup_hints: ["hint"],
    pending_propagation: { count: 4, zitadel_reachable: false },
    drift: { count: 3, top: [] },
    unreconciled_targets: [{ target: "truenas", since: "2026-08-01T00:00:00Z" }],
    merge_findings: 2,
  };

  // The general guard: every key the type promises has to come out the other
  // side. Add a field to `GovernanceSummary`, forget the mapper, fail here.
  it("carries every field the type declares", () => {
    const mapped = mapGovernanceSummary(wire);
    for (const key of Object.keys(wire) as Array<keyof GovernanceSummary>) {
      expect(mapped[key], `${key} was dropped in transit`).toBeDefined();
    }
  });

  it("carries the value, not just the key", () => {
    const mapped = mapGovernanceSummary(wire);
    expect(mapped.merge_findings).toBe(2);
    expect(mapped.drift.count).toBe(3);
    expect(mapped.pending_propagation.zitadel_reachable).toBe(false);
  });

  // Absent is zero, not undefined: Today does arithmetic over these, and one
  // undefined turns the whole headline into NaN.
  it("substitutes a usable value for anything the server omitted", () => {
    const mapped = mapGovernanceSummary(undefined);
    expect(mapped.merge_findings).toBe(0);
    expect(mapped.pending_requests).toEqual([]);
    expect(mapped.drift.count).toBe(0);
    expect(mapped.pending_propagation.zitadel_reachable).toBe(true);
  });

  // A server that answers with the wrong shape must not hand a screen
  // something it will call .length or .map on.
  it("refuses a scalar where a list is promised", () => {
    const mapped = mapGovernanceSummary({
      pending_requests: 3 as unknown as GovernanceSummary["pending_requests"],
      merge_findings: "2" as unknown as number,
    });
    expect(mapped.pending_requests).toEqual([]);
    expect(mapped.merge_findings).toBe(0);
  });
});

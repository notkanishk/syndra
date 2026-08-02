import { describe, expect, it } from "vitest";

import { daysUntilTermEnd, nextTermEnd } from "@/lib/term";

describe("nextTermEnd", () => {
  it("returns the May boundary from earlier in the year", () => {
    expect(nextTermEnd(new Date(2026, 1, 3))).toEqual(new Date(2026, 4, 18));
  });

  it("returns the December boundary once May has passed", () => {
    expect(nextTermEnd(new Date(2026, 7, 2))).toEqual(new Date(2026, 11, 18));
  });

  it("rolls into next year once December has passed", () => {
    expect(nextTermEnd(new Date(2026, 11, 20))).toEqual(new Date(2027, 4, 18));
  });

  it("never resolves to zero days, even on the boundary itself", () => {
    expect(daysUntilTermEnd(new Date(2026, 4, 18, 12))).toBeGreaterThanOrEqual(1);
  });
});

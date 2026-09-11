import { describe, expect, it } from "vitest";

import { holdersLine } from "@/lib/holders";

describe("holdersLine", () => {
  it("reads as unchecked when Zitadel has never been read", () => {
    expect(holdersLine(5, undefined, undefined)).toEqual({
      headline: "5",
      note: "not checked yet",
      tone: "muted",
    });
  });

  it("says confirmed when observed matches what's recorded", () => {
    expect(holdersLine(5, 5, 5)).toEqual({ headline: "5", note: "confirmed", tone: "ok" });
  });

  it("says nothing extra for a quiet zero", () => {
    // observed = 0, nothing recorded to explain either — not "confirmed",
    // just silent. Mutation-sensitive: a `>= 0` in place of `> 0` would push
    // "confirmed" here.
    expect(holdersLine(0, 0, 0)).toEqual({ headline: "0", note: "", tone: "ok" });
  });

  it("shows both parts when access is both unexplained and not fully given, danger winning the tone", () => {
    // confirmed=3 of a given 5, observed=5: 2 are unexplained (observed
    // outruns confirmed) AND 2 of what was given was never confirmed either
    // — both parts say something true, danger outranks warn.
    expect(holdersLine(5, 3, 5)).toEqual({
      headline: "5",
      note: "2 unexplained · 2 not in Zitadel yet",
      tone: "danger",
    });
  });

  it("treats a never-confirmed observed count as fully unexplained", () => {
    // confirmed is undefined (never confirmed at all), not zero — the base
    // must fall back to 0, not to `given`. Mutation-sensitive: `confirmed ??
    // given` here would silently hide real drift.
    expect(holdersLine(5, undefined, 3)).toEqual({
      headline: "3",
      note: "3 unexplained · 5 not in Zitadel yet",
      tone: "danger",
    });
  });

  it("flags what's given but not yet in Zitadel, when nothing is unexplained", () => {
    expect(holdersLine(4, 1, 1)).toEqual({
      headline: "1",
      note: "3 not in Zitadel yet",
      tone: "warn",
    });
  });

  it("never lets a negative unexplained or missing count leak into the note", () => {
    // observed and given both fall below what's confirmed — a stale
    // confirmation, not new access. Mutation-sensitive: dropping the `> 0`
    // guards would print negative counts instead of falling through to
    // "confirmed".
    expect(holdersLine(1, 3, 1)).toEqual({ headline: "1", note: "confirmed", tone: "ok" });
  });
});

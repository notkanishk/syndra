import { describe, expect, it } from "vitest";

import { keptEnds, stillEnds } from "@/lib/keep-ends";

// A settled request to keep a role remembers the end they had and the end
// approving set (migration 000063), so its result is said from the request and
// not from a grant that may have moved since.

describe("keptEnds", () => {
  it("both ends: now ends instead of", () => {
    expect(keptEnds({ ended_before: "2099-12-18T12:00:00Z", ends_after: "2100-01-18T12:00:00Z" })).toBe(
      "It now ends 18 Jan 2100 instead of 18 Dec 2099.",
    );
  });

  it("a role that had lapsed has nothing to say it replaced", () => {
    expect(keptEnds({ ended_before: null, ends_after: "2100-01-18T12:00:00Z" })).toBe("It now ends 18 Jan 2100.");
    expect(keptEnds({ ends_after: "2100-01-18T12:00:00Z" })).toBe("It now ends 18 Jan 2100.");
  });

  it("no end after approving is no end date", () => {
    expect(keptEnds({ ended_before: "2099-12-18T12:00:00Z", ends_after: null })).toBe(
      "It now has no end date, instead of ending 18 Dec 2099.",
    );
  });

  it("an older request with neither recorded says nothing", () => {
    expect(keptEnds({})).toBeUndefined();
    expect(keptEnds({ ended_before: null, ends_after: null })).toBeUndefined();
  });
});

describe("stillEnds", () => {
  it("the end they had, while it is still ahead", () => {
    expect(stillEnds("2099-12-18T12:00:00Z")).toBe("It still ends 18 Dec 2099.");
  });

  it("an end that has passed since doesn't still end anywhere", () => {
    expect(stillEnds("2020-01-01T00:00:00Z")).toBeUndefined();
    expect(stillEnds(null)).toBeUndefined();
    expect(stillEnds(undefined)).toBeUndefined();
  });
});

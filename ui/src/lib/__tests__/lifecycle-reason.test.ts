import { describe, expect, it } from "vitest";

import { lifecycleWhy } from "@/lib/lifecycle-reason";

describe("the reason an add-on is not simply active", () => {
  it("turns the add-on's own codes into sentences, never showing the code", () => {
    expect(lifecycleWhy("startup")).toBe("It was started this way by its configuration.");
    expect(lifecycleWhy("shutting down")).toBe("It is shutting down.");
    expect(lifecycleWhy("configured lifecycle state was not recognised")).toMatch(/not recognised, so it started read-only/);
  });

  it("quotes what an operator typed, as given", () => {
    expect(lifecycleWhy("rotating the API key")).toBe("Somebody set that on purpose: \u201crotating the API key\u201d.");
  });

  it("still says it was on purpose when no reason came with it", () => {
    expect(lifecycleWhy(undefined)).toBe("Somebody set that on purpose.");
    expect(lifecycleWhy("  ")).toBe("Somebody set that on purpose.");
  });
});

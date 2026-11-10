import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { humanizeKey, roleName } from "@/lib/names";

describe("roleName", () => {
  it("is the display name when there is one", () => {
    expect(roleName("Trained operator", "laser_trained")).toBe("Trained operator");
  });

  it("is the key made readable when there is none, null, undefined or empty", () => {
    for (const none of [undefined, null, ""]) expect(roleName(none, "laser_trained")).toBe("Laser Trained");
  });

  it("humanizeKey handles dashes, underscores and an empty key", () => {
    expect(humanizeKey("shop-floor_lead")).toBe("Shop Floor Lead");
    expect(humanizeKey("")).toBe("");
  });

  // The Go `roleLabel` (backend/internal/services/views.go) reads this same file
  // in TestRoleLabelMatchesHumanizeKey: the two can never disagree.
  it("humanizeKey matches the shared fixture the backend reads", () => {
    const cases = JSON.parse(readFileSync(join(__dirname, "..", "__fixtures__", "humanize-key.json"), "utf8")) as Array<{ key: string; label: string }>;
    expect(cases.length).toBeGreaterThan(5);
    for (const c of cases) expect(humanizeKey(c.key), JSON.stringify(c.key)).toBe(c.label);
  });
});

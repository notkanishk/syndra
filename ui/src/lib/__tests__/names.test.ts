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
});

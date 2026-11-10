import { describe, expect, it } from "vitest";

import { asOfPanel, pastHeld } from "@/components/insights/asof";
import type { Model } from "@/components/insights/model";
import type { InsightsAsOf } from "@/lib/queries/useInsights";

// A map that knows one role, "p:r", as node "n1".
const model = { roleId: (p: string, r: string) => (p === "p" && r === "r" ? "n1" : null), nameOf: () => "Laser" } as unknown as Model;

const picture = (role_people: Record<string, number> | null): InsightsAsOf => ({
  kind: "picture",
  day: "2026-10-12",
  topology: { nodes: [], edges: [] },
  role_people,
  first_day: "2026-10-10",
  changes_since: 3,
});

describe("as of a day Zitadel was never read", () => {
  it("has no people to count for any role", () => {
    expect(pastHeld(model, null).size).toBe(0);
  });

  it("says it wasn't checked, and sets nothing against today's numbers", () => {
    const panel = asOfPanel("2026-10-12", picture(null), { model, held: new Map([["n1", 5]]) }, pastHeld(model, null));
    const text = panel.lines.map((l) => `${l.lead ?? ""} ${l.text}`).join("\n");
    expect(text).toMatch(/wasn't checked/);
    expect(text).not.toMatch(/reached/);
  });

  it("still says what changed when that day was read", () => {
    const read = picture({ "p:r": 2 });
    const panel = asOfPanel("2026-10-12", read, { model, held: new Map([["n1", 5]]) }, pastHeld(model, read.kind === "picture" ? read.role_people : null));
    const text = panel.lines.map((l) => l.text).join("\n");
    expect(text).toMatch(/reached 2 people. It reaches 5 now./);
    expect(text).not.toMatch(/wasn't checked/);
  });
});

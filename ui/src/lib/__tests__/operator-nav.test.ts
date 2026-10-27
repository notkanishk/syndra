import { describe, expect, it } from "vitest";

import { ADVANCED_NAV, BASIC_NAV, navLeaves, operatorNav, targetNav } from "@/lib/nav";

/**
 * A-08 §3: an operator's rail is Advanced's whole tree in both views, with what
 * Basic lacks marked, so the rail can fold exactly those parts and nothing else.
 */
describe("operatorNav", () => {
  it("is Advanced's tree in Advanced's order, whichever view is on", () => {
    const shape = (entries: ReturnType<typeof operatorNav>) =>
      entries.map((e) => (e.kind === "leaf" ? e.href : `${e.label}:${e.children.map((c) => c.href).join(",")}`));
    expect(shape(operatorNav([]))).toEqual(shape(ADVANCED_NAV));
    expect(shape(operatorNav(["truenas"]))).toEqual(shape(targetNav(["truenas"])));
  });

  it("marks a row Advanced-only exactly when Basic does not have it", () => {
    const basic = new Set(navLeaves(BASIC_NAV).map((leaf) => leaf.href));
    for (const leaf of navLeaves(operatorNav(["truenas", "unifi"]))) {
      expect(Boolean(leaf.advancedOnly), leaf.href).toBe(!basic.has(leaf.href));
    }
  });

  it("leaves Basic's own rows and groups unmarked, so they never fold", () => {
    const entries = operatorNav([]);
    const named = (label: string) => entries.find((e) => e.label === label);
    for (const label of ["Home", "People", "Access", "Requests"]) {
      expect(named(label)?.advancedOnly, label).toBeFalsy();
    }
    const access = named("Access");
    expect(access?.kind === "group" && access.children.map((c) => c.advancedOnly)).toEqual([false, false, false]);
  });

  it("marks a group Basic lacks as a whole, and a target's row with it", () => {
    const entries = operatorNav(["truenas"]);
    for (const label of ["Bundles", "Automatic rules", "Insights", "Changes", "Review", "System", "Makerspace settings"]) {
      expect(entries.find((e) => e.label === label)?.advancedOnly, label).toBe(true);
    }
    const system = entries.find((e) => e.kind === "group" && e.label === "System");
    const truenas = system?.kind === "group" ? system.children.find((c) => c.label === "TrueNAS") : undefined;
    expect(truenas?.advancedOnly).toBe(true);
  });

  it("does not mark the static trees it is derived from", () => {
    operatorNav([]);
    expect(navLeaves(ADVANCED_NAV).some((leaf) => "advancedOnly" in leaf)).toBe(false);
  });
});

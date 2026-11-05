import { describe, expect, it } from "vitest";

import { ADVANCED_NAV, BASIC_NAV, isAdvancedOnly, navLeaves } from "@/lib/nav";

// A-20 P10: which links switch the view is read from the two rails, never restated.
describe("isAdvancedOnly", () => {
  it("is false for every page Basic's rail has, and for the pages under them", () => {
    for (const leaf of navLeaves(BASIC_NAV)) expect(isAdvancedOnly(leaf.href), leaf.href).toBe(false);
    expect(isAdvancedOnly("/users/u1")).toBe(false);
    expect(isAdvancedOnly("/projects/p1/roles/operator")).toBe(false);
    expect(isAdvancedOnly("/applications/a1")).toBe(false);
  });

  it("is true for every page only Advanced's rail has", () => {
    const basic = new Set(navLeaves(BASIC_NAV).map((leaf) => leaf.href));
    const only = navLeaves(ADVANCED_NAV).filter((leaf) => !basic.has(leaf.href));
    expect(only.length).toBeGreaterThan(8);
    for (const leaf of only) expect(isAdvancedOnly(leaf.href), leaf.href).toBe(true);
  });

  it("follows a connected system's own pages, which the rail adds at run time", () => {
    expect(isAdvancedOnly("/system/targets/truenas")).toBe(true);
    expect(isAdvancedOnly("/system/targets/truenas/mappings")).toBe(true);
    expect(isAdvancedOnly("/system/service-accounts/u1")).toBe(true);
  });

  it("judges the path, never the query or the hash", () => {
    expect(isAdvancedOnly("/governance/pending?person=u1")).toBe(true);
    expect(isAdvancedOnly("/zitadel#claims-delivery")).toBe(true);
    expect(isAdvancedOnly("/users?filter=expiring")).toBe(false);
  });

  it("leaves pages outside both rails, and other sites, alone", () => {
    expect(isAdvancedOnly("/me")).toBe(false);
    expect(isAdvancedOnly("/auth/zitadel?then=close")).toBe(false);
    expect(isAdvancedOnly("https://example.org/audit")).toBe(false);
  });
});

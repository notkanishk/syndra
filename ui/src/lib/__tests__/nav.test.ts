import { describe, expect, it } from "vitest";

import {
  ADVANCED_NAV,
  BASIC_NAV,
  MEMBER_NAV,
  crumbsFor,
  leafMatches,
  navFor,
  navLeaves,
  memberMayVisit,
} from "@/lib/nav";

describe("navigation contract", () => {
  // The prohibition this whole file exists to enforce: the old rail injected a
  // Drift section at the top when its count went non-zero, pushing every other
  // item down under the operator's cursor.
  it("Advanced appends to Basic without reordering or renaming anything", () => {
    const basic = navLeaves(BASIC_NAV).map((leaf) => leaf.label);
    const advanced = navLeaves(ADVANCED_NAV).map((leaf) => leaf.label);

    expect(advanced.slice(0, basic.length)).toEqual(basic);
    expect(advanced.length).toBeGreaterThan(basic.length);
  });

  it("puts Today first in both operator views", () => {
    expect(navLeaves(BASIC_NAV)[0].label).toBe("Today");
    expect(navLeaves(ADVANCED_NAV)[0].label).toBe("Today");
  });

  it("gives members two destinations and no Today", () => {
    const labels = navLeaves(MEMBER_NAV).map((leaf) => leaf.label);
    expect(labels).toEqual(["My access", "Requests"]);
    expect(navFor("member")).toBe(MEMBER_NAV);
  });

  it("routes every badge to a destination, never to a tab", () => {
    for (const leaf of navLeaves(ADVANCED_NAV)) {
      if (!leaf.indicator) continue;
      expect(leaf.href, `${leaf.label} badge must point at a page`).not.toContain("?");
    }
  });

  it("gives each badge the tone its meaning calls for", () => {
    const byLabel = new Map(navLeaves(ADVANCED_NAV).map((leaf) => [leaf.label, leaf]));
    // Work is accent, a deadline is amber, something already wrong is red.
    expect(byLabel.get("Requests")?.tone ?? "accent").toBe("accent");
    expect(byLabel.get("Expiring access")?.tone).toBe("warn");
    expect(byLabel.get("Unexplained access")?.tone).toBe("danger");
  });

  it("routes /grants nowhere — it ceased to exist as a destination", () => {
    const hrefs = navLeaves(ADVANCED_NAV).map((leaf) => leaf.href);
    expect(hrefs).not.toContain("/grants");
  });

  it("gives every route exactly one home", () => {
    const hrefs = navLeaves(ADVANCED_NAV).map((leaf) => leaf.href);
    expect(new Set(hrefs).size).toBe(hrefs.length);
  });
});

describe("active-route matching", () => {
  const byLabel = new Map(navLeaves(ADVANCED_NAV).map((leaf) => [leaf.label, leaf]));

  it("keeps a project detail under Projects and a role detail under Roles", () => {
    const projects = byLabel.get("Projects")!;
    const roles = byLabel.get("Roles")!;

    expect(leafMatches(projects, "/projects")).toBe(true);
    expect(leafMatches(projects, "/projects/pLaser")).toBe(true);
    // The one a prefix match gets wrong: a role page lives under a project URL
    // but belongs to Roles.
    expect(leafMatches(projects, "/projects/pLaser/roles/trained")).toBe(false);
    expect(leafMatches(roles, "/projects/pLaser/roles/trained")).toBe(true);
  });

  it("keeps a person's page under People", () => {
    const people = byLabel.get("People")!;
    expect(leafMatches(people, "/users")).toBe(true);
    expect(leafMatches(people, "/users/u_2f81")).toBe(true);
  });

  it("does not light Today up on every route", () => {
    const today = byLabel.get("Today")!;
    expect(leafMatches(today, "/")).toBe(true);
    expect(leafMatches(today, "/users")).toBe(false);
  });
});

describe("breadcrumbs", () => {
  it("carries the group for a nested destination", () => {
    expect(crumbsFor("/roles", "advanced")).toEqual([
      { label: "Access" },
      { label: "Roles", href: "/roles" },
    ]);
  });

  it("keeps the parent context on a detail route", () => {
    expect(crumbsFor("/users/u_2f81", "basic")).toEqual([{ label: "People", href: "/users" }]);
    expect(crumbsFor("/projects/pLaser/roles/trained", "advanced")).toEqual([
      { label: "Access" },
      { label: "Roles", href: "/roles" },
    ]);
  });

  it("is empty for a route the audience cannot reach", () => {
    // A member has no Audit destination, so no crumb claims they do.
    expect(crumbsFor("/audit", "member")).toEqual([]);
  });
});

describe("member reachability", () => {
  it("allows only the two member destinations", () => {
    expect(memberMayVisit("/")).toBe(true);
    expect(memberMayVisit("/requests")).toBe(true);
    for (const route of ["/users", "/bundles", "/governance/drift", "/applications", "/audit"]) {
      expect(memberMayVisit(route), `${route} must not be reachable`).toBe(false);
    }
  });
});

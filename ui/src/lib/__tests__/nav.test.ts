import { existsSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  ADVANCED_NAV,
  BASIC_NAV,
  MEMBER_NAV,
  MEMBER_ROUTES,
  crumbsFor,
  leafMatches,
  navFor,
  navLeaves,
  targetNav,
  targetLabel,
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

  it("puts Home first in both operator views", () => {
    expect(navLeaves(BASIC_NAV)[0].label).toBe("Home");
    expect(navLeaves(ADVANCED_NAV)[0].label).toBe("Home");
  });

  it("gives members three destinations and no Home", () => {
    const labels = navLeaves(MEMBER_NAV).map((leaf) => leaf.label);
    expect(labels).toEqual(["My access", "Requests", "Network storage"]);
    expect(navFor("member")).toBe(MEMBER_NAV);
  });

  // 10.2 — the storage row is present for every member whatever they can reach.
  // Gating it on entitlement would make the rail move as somebody's roles
  // change, and it would answer the wrong question: a member without access is
  // asking whether they can get it, and a missing row does not answer that.
  it("keeps the storage row for a member with no infrastructure access", () => {
    const storage = navLeaves(MEMBER_NAV).find((leaf) => leaf.href === "/storage");
    expect(storage, "every member sees the storage row").toBeDefined();
    expect(storage?.indicator, "a badge would make it move in response to data").toBeUndefined();
    expect(MEMBER_ROUTES).toContain("/storage");
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

  it("does not light Home up on every route", () => {
    const today = byLabel.get("Home")!;
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
  // Every row the member rail renders, not a hand-written subset: a
  // destination offered and then refused is the bug this pairing exists to
  // prevent, and a literal list here would drift the same way the middleware's
  // did.
  it("allows every destination the member rail offers", () => {
    for (const leaf of navLeaves(MEMBER_NAV)) {
      expect(memberMayVisit(leaf.href), `${leaf.label} is offered and must be reachable`).toBe(
        true,
      );
    }
    expect(MEMBER_ROUTES).toEqual(navLeaves(MEMBER_NAV).map((leaf) => leaf.href));
  });

  it("refuses every operator destination", () => {
    for (const route of ["/users", "/bundles", "/governance/drift", "/applications", "/audit"]) {
      expect(memberMayVisit(route), `${route} must not be reachable`).toBe(false);
    }
  });

  // `/storage` grows a per-target child as soon as a second add-on ships.
  it("admits a child of a member destination", () => {
    expect(memberMayVisit("/storage/truenas")).toBe(true);
    expect(memberMayVisit("/requests/req_01")).toBe(true);
  });

  // The prefix must end at a segment boundary, or `/storage-admin` walks in on
  // the strength of sharing seven characters with a route a member may reach.
  it("does not admit a route that merely starts with one", () => {
    expect(memberMayVisit("/storage-admin")).toBe(false);
    expect(memberMayVisit("/requests-review")).toBe(false);
    expect(memberMayVisit("/users"), "root must not match by prefix").toBe(false);
  });
});

// 9.13/9.14 — the per-target System rows come from the deployment, not from what
// the operator can see.
describe("the target rows", () => {
  it("renders one row per registered add-on, whatever the data says", () => {
    const entries = targetNav(["truenas", "unifi"]);
    const system = entries.find((e) => e.kind === "group" && e.label === "System");
    expect(system?.kind).toBe("group");
    const labels = system?.kind === "group" ? system.children.map((c) => c.label) : [];

    // Present, and in the System group beside the identity provider. The
    // deployment registered them; nothing about a person's access is consulted.
    expect(labels).toContain("TrueNAS");
    expect(labels).toContain("UniFi Access");
    // And the identity provider stays first: switching views appends, and so
    // does registering a target. Nothing already there moves.
    expect(labels[0]).toBe("Identity provider");
  });

  it("has no row for the bridge that no longer exists", () => {
    for (const entries of [ADVANCED_NAV, targetNav(["truenas"])]) {
      const labels = navLeaves(entries).map((leaf) => leaf.label);
      expect(labels, "a nav row for a deleted subsystem is worse than a missing one")
        .not.toContain("Hardware sync");
      const hrefs = navLeaves(entries).map((leaf) => leaf.href);
      expect(hrefs).not.toContain("/system/hardware-sync");
    }
  });

  it("leaves the rail alone when the deployment registered nothing", () => {
    expect(targetNav([])).toBe(ADVANCED_NAV);
  });

  it("names a target once, from one place", () => {
    expect(targetLabel("truenas")).toBe("TrueNAS");
    // An unknown target still gets a name rather than an id, so shipping an
    // add-on does not require editing the navigation to make it readable.
    expect(targetLabel("proxmox")).toBe("Proxmox");
  });
});

/**
 * §17 — and the route is gone too, not merely unlinked.
 *
 * Removing the nav row was half of retiring the sync service. The page stayed,
 * importing a query that polls `/api/v1/intents` every five seconds — a route
 * the same change deleted — so a bookmarked tab polled a 404 forever and the
 * ledger recorded the item as done. Unlinking is not deleting; a URL somebody
 * saved is still a URL.
 */
describe("the retired sync surface", () => {
  it("has no page and no query left behind the removed nav row", () => {
    for (const orphan of ["../../app/system/hardware-sync/page.tsx", "../../lib/queries/useIntents.ts"]) {
      expect(existsSync(resolve(__dirname, orphan)), `${orphan} still exists`).toBe(false);
    }
  });
});

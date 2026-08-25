import { describe, expect, it } from "vitest";

import { ADVANCED_NAV, BASIC_NAV, MEMBER_NAV, targetNav } from "@/lib/nav";
import {
  destinationMatches,
  destinationsWantingAttention,
  navShape,
  tonesInPlay,
  touchDestinations,
} from "@/lib/touch-nav";
import { loudestTone } from "@/components/shell/navTones";

describe("the rail becomes destinations without gaining or losing one", () => {
  it("keeps a member's three", () => {
    const destinations = touchDestinations(MEMBER_NAV);
    expect(destinations.map((d) => d.label)).toEqual(["My access", "Requests", "Network storage"]);
    expect(navShape(destinations)).toBe("tabs");
  });

  // Basic has four entries, one of which is a group of three. The group is one
  // destination or the bar has six tabs and the rule that produced it is gone.
  it("collapses Basic's Access group into one destination that keeps the group's name", () => {
    const destinations = touchDestinations(BASIC_NAV);
    expect(destinations).toHaveLength(4);
    expect(destinations.map((d) => d.label)).toEqual(["Home", "People", "Access", "Requests"]);
    expect(navShape(destinations)).toBe("tabs");

    const access = destinations[2];
    expect(access.href, "the tab lands on the group's first child").toBe("/projects");
    expect(access.children?.map((c) => c.label)).toEqual(["Projects", "Roles", "Apps"]);
  });

  it("sends Advanced to the sheet, because eight destinations are not four", () => {
    const destinations = touchDestinations(ADVANCED_NAV);
    expect(destinations).toHaveLength(8);
    expect(navShape(destinations)).toBe("sheet");
  });

  // A deployment registering add-ons must not be able to grow the tab bar.
  it("cannot be pushed past the tab-bar limit by a deployment's add-ons", () => {
    const withTargets = touchDestinations(targetNav(["truenas", "unifi"]));
    expect(navShape(withTargets)).toBe("sheet");
  });
});

describe("a destination knows when it is the one you are looking at", () => {
  const destinations = touchDestinations(BASIC_NAV);
  const access = destinations[2];

  it("matches a group when any child does", () => {
    expect(destinationMatches(access, "/roles")).toBe(true);
    expect(destinationMatches(access, "/applications")).toBe(true);
    expect(destinationMatches(access, "/projects")).toBe(true);
  });

  it("matches a leaf on its own pattern", () => {
    expect(destinationMatches(destinations[1], "/users/u_123")).toBe(true);
    expect(destinationMatches(destinations[1], "/requests")).toBe(false);
  });

  it("does not let Home match everything", () => {
    expect(destinationMatches(destinations[0], "/")).toBe(true);
    expect(destinationMatches(destinations[0], "/users")).toBe(false);
  });
});

describe("what the sheet's entry reports", () => {
  const destinations = touchDestinations(ADVANCED_NAV);

  // Three findings plus eleven expiries plus three holds is seventeen of
  // nothing: no single action reduces it and no operator can act on it. The
  // count is of places to go.
  it("counts destinations wanting attention, never items outstanding", () => {
    const counts = { drift: 3, expiring_grants: 11, holds_due: 3, pending_requests: 0 };
    // Unexplained access, Expiring access and Holds due all live under Review,
    // so three non-zero indicators are ONE destination wanting attention.
    expect(destinationsWantingAttention(destinations, counts)).toBe(1);
  });

  it("counts a second destination when a second group lights up", () => {
    const counts = { drift: 3, pending_propagation: 2 };
    expect(destinationsWantingAttention(destinations, counts)).toBe(2);
  });

  it("reports nothing when nothing is outstanding", () => {
    expect(destinationsWantingAttention(destinations, {})).toBe(0);
    expect(destinationsWantingAttention(destinations, undefined)).toBe(0);
  });

  it("shows a collapsed group the loudest tone it is carrying, not a sum", () => {
    const review = destinations.find((d) => d.label === "Review")!;
    // Expiring access is warn, Unexplained access is danger. Danger wins.
    expect(loudestTone(tonesInPlay(review, { expiring_grants: 11, drift: 1 }))).toBe("danger");
    // With only the deadline outstanding, the dot is amber and says so.
    expect(loudestTone(tonesInPlay(review, { expiring_grants: 11 }))).toBe("warn");
    expect(loudestTone(tonesInPlay(review, {}))).toBeNull();
  });
});

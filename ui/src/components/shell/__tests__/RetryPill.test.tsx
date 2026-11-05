// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RetryPill } from "@/components/shell/RetryPill";
import { retryBadge } from "@/components/shell/navTones";
import { BASIC_NAV, ADVANCED_NAV, type NavEntry, type NavLeaf } from "@/lib/nav";
import { tonesInPlay, touchDestinations } from "@/lib/touch-nav";

/**
 * Retries in the rail (owner, 5 Nov 2026): Basic has no Pending changes row,
 * so Home carries the retry pill; Advanced's Pending changes carries it too.
 * Amber with the count while Syndra retries; danger once it gave up on a
 * change nobody has marked seen.
 */
const leaves = (nav: NavEntry[]): NavLeaf[] => nav.flatMap((e) => (e.kind === "leaf" ? [e] : e.children));

describe("the retry pill", () => {
  it("has a seat in Basic (Home) and in Advanced (Home and Pending changes)", () => {
    expect(leaves(BASIC_NAV).filter((l) => l.retries).map((l) => l.label)).toEqual(["Home"]);
    expect(leaves(ADVANCED_NAV).filter((l) => l.retries).map((l) => l.label)).toEqual(["Home", "Pending changes"]);
  });

  it("is amber with the count while retrying, danger once given up, absent otherwise", () => {
    expect(retryBadge({ retrying: 2, gave_up: 0 })).toMatchObject({ count: 2, tone: "warn" });
    expect(retryBadge({ retrying: 2, gave_up: 1 })).toMatchObject({ count: 1, tone: "danger" });
    expect(retryBadge({ retrying: 0, gave_up: 0 })).toBeNull();
    expect(retryBadge(undefined)).toBeNull();
  });

  it("says it to a screen reader, not by colour alone", () => {
    render(<RetryPill counts={{ retrying: 3 }} />);
    expect(screen.getByText(/Syndra is sending 3 changes again on its own/)).toBeInTheDocument();
  });

  it("lights a collapsed Changes group on a phone", () => {
    const changes = touchDestinations(ADVANCED_NAV).find((d) => d.label === "Changes")!;
    expect(tonesInPlay(changes, { pending_propagation: 0, retrying: 1 })).toContain("warn");
    expect(tonesInPlay(changes, { pending_propagation: 0, gave_up: 1 })).toContain("danger");
  });
});

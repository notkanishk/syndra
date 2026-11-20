// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { lampRoom, type Indicators } from "@/lib/queries/useIndicators";
import { css } from "./cssTokens";

// A-25 moment 3, the lamp breathes (accepted: "rest recommended seem good"): while the rail holds a
// live reading the mark's glow breathes once every ten seconds; amber and quicker while Zitadel isn't
// answering; still without a reading, and for a member until their reading is proven.

const reading: Indicators = {
  pending_requests: 0,
  expiring_grants: 0,
  holds_due: 0,
  pending_propagation: 0,
  drift: 0,
  unconfirmed_revocations: 0,
  revocations_escalated: false,
  zitadel_reachable: true,
};

/** The block a selector opens, braces balanced; a selector that is not there fails, never vacuously. */
function braced(source: string, opener: string): string {
  const start = source.indexOf(opener);
  if (start < 0) throw new Error(`not in the CSS: ${opener}`);
  let depth = 0;
  for (let i = source.indexOf("{", start); i < source.length; i++) {
    depth += source[i] === "{" ? 1 : source[i] === "}" ? -1 : 0;
    if (depth === 0) return source.slice(start, i + 1);
  }
  return source.slice(start);
}
/** The CSS with every block gated on `prefers-reduced-motion: no-preference` taken out. */
function ungated(source: string): string {
  const at = source.search(/@media [^{]*\(prefers-reduced-motion: no-preference\) \{/);
  return at < 0 ? source : ungated(source.slice(0, at) + source.slice(at + braced(source.slice(at), "@media").length));
}
const MOTION = "@media (forced-colors: none) and (prefers-reduced-motion: no-preference) {\n    .syndra-mark";
const breathing = braced(css, MOTION);
const glow = braced(css, '.syndra-mark[data-room]::before {');

describe("the lamp breathes only on a live reading", () => {
  it("watches on a live reading, waits while Zitadel isn't answering, and is still otherwise", () => {
    expect(lampRoom(true, { data: reading, isPlaceholderData: false })).toBe("watch");
    expect(lampRoom(true, { data: { ...reading, zitadel_reachable: false }, isPlaceholderData: false })).toBe("waiting");
    // No reading: before the first (the placeholder's zeros), none at all, or the read failing.
    expect(lampRoom(true, { data: reading, isPlaceholderData: true })).toBeUndefined();
    expect(lampRoom(true, { data: undefined, isPlaceholderData: false })).toBeUndefined();
    expect(lampRoom(true, { data: reading, isPlaceholderData: false, isError: true })).toBeUndefined();
    // Offline: the poll is paused, and the last reading is not a live one.
    expect(lampRoom(true, { data: reading, isPlaceholderData: false, fetchStatus: "paused" })).toBeUndefined();
    expect(lampRoom(true, { data: reading, isPlaceholderData: false, fetchStatus: "idle" })).toBe("watch");
    // A member's lamp stays still until their reading is proven (accepted).
    expect(lampRoom(false, { data: reading, isPlaceholderData: false })).toBeUndefined();
  });

  it("breathes only on a lit mark: once every --door-breath while watching, on --duration-breathe while waiting", () => {
    expect(breathing).toMatch(/\.syndra-mark\[data-room="watch"\]::before \{\s*animation: lamp calc\(var\(--door-breath\) \/ 2\) var\(--ease-breathe\) infinite alternate;/);
    expect(breathing).toMatch(/\.syndra-mark\[data-room="waiting"\]::before \{\s*animation: breathe var\(--duration-breathe\) var\(--ease-breathe\) infinite alternate;/);
    // The glow is drawn only on a lit mark; the mark as built has no glow and nothing that moves.
    expect(css).not.toMatch(/\.syndra-mark::before \{\s*content/);
    expect(braced(css, "@keyframes lamp {")).toMatch(/from \{\s*opacity: 0\.28;\s*scale: 0\.76;\s*\}\s*to \{\s*opacity: 1;\s*scale: 1;/);
  });
});

describe("what the lamp draws", () => {
  it("is amber while Zitadel isn't answering: the glow is the dot's own colour, and waiting's dot is --warn", () => {
    expect(braced(css, ".syndra-mark[data-room] {")).toMatch(/--pool-c: var\(--mark-dot\);\s*--pool-k: 36%;/);
    expect(braced(css, '.syndra-mark[data-room="waiting"] {')).toMatch(/--mark-dot: var\(--warn\);/);
    expect(glow).toMatch(/background: radial-gradient\(closest-side, var\(--pool-stops\)\);/);
  });

  it("is still under reduced motion: at rest, 0.72, and its breath only for a visitor who has not asked for less", () => {
    expect(glow).toMatch(/opacity: 0\.72;/);
    expect(glow).not.toMatch(/animation/);
    // No glow breathes outside a block gated on no-preference: the lamp's, and every light's since (the trail's).
    expect(ungated(css)).not.toMatch(/::before \{\s*animation: (lamp|breathe)/);
    expect(ungated(css)).not.toContain(breathing);
  });

  it("is gone under forced colours", () => {
    expect(braced(css, "@media (forced-colors: active) {\n    .syndra-mark::before")).toMatch(/display: none;/);
    expect(MOTION).toContain("(forced-colors: none)");
  });

  it("reaches 0.85 mark-widths out (the layout check's `lamp` rule measures it against every clip)", () => {
    expect(glow).toMatch(/inset: calc\(var\(--mark-size\) \* -0\.85\);/);
  });
});

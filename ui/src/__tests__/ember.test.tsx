// @vitest-environment jsdom
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";
import { StatusDot } from "@/components/ui/Badge";
import { SRC, block, colour, css, over, ratio, type RGBA } from "./cssTokens";

// A-25 moment 7, the ember (owner, 20 Nov 2026: "yes to recommended", the ember last). The lime dot of a
// clear queue holds a low, edgeless glow at rest; when the reader's own act cleared the queue it opens
// once. It replaces `bloom`, the flat disc with an edge. Whether an act was the reader's own is decided
// where `bloom` was decided (`useQueueCleared`, each page's own gate): its tests are HomeHandover,
// everythingSent, ownActClear, TargetOverviewMotion and MergeFindingsMotion.

const glow = (() => {
  const at = css.indexOf("@utility dot-glow");
  expect(at, "@utility dot-glow not in globals.css").toBeGreaterThan(-1);
  return css.slice(at, css.indexOf("\n}\n", at));
})();

describe("the ember at rest", () => {
  it("is a pool of the dot's own colour at 44%, 17px out from the 6px dot, held at 0.8, still", () => {
    expect(glow).toMatch(/--pool-k: 44%;/);
    expect(glow).toMatch(/--pool-c: var\(--dot-light, currentColor\);/);
    expect(glow).toMatch(/inset: -17px;/);
    expect(glow).toMatch(/&\.ember::before \{\s*opacity: 0\.8;/);
    // 6px dot + 2 x 17px: spent 20px from the dot's centre, inside any card it sits in.
    expect(glow).toMatch(/radial-gradient\(closest-side, var\(--pool-stops\)\)/);
    // No edge: nothing in it but the one light's stops, and no hex.
    expect(glow).not.toMatch(/#[0-9a-f]{3,6}\b|border|box-shadow/i);
    // Under the words, never a fill behind them.
    expect(glow).toMatch(/z-index: -1;/);
  });

  it("is drawn for a resolved queue's dot, still and unopened, and for nothing that is not resolved", () => {
    const rest = render(<EmptyState resolved title="Done." />);
    const dot = rest.container.querySelector("[data-dot]")!;
    expect(dot).toHaveClass("dot-glow", "ember");
    expect(dot).toHaveAttribute("data-ember");
    expect(dot).not.toHaveClass("ember-open");
    // The row is isolated, so the pool is behind its words and above the card.
    expect(dot.parentElement).toHaveClass("isolate");
    rest.unmount();
    // A search that missed and a read that failed are not resolved: no dot, so no ember.
    expect(render(<EmptyState title="Nothing matches." />).container.querySelector("[data-dot]")).toBeNull();
  });

  it("is never drawn on a ring, which says this is not a fresh read", () => {
    const { container } = render(<StatusDot tone="healthy" ember ring bloom />);
    const dot = container.querySelector("[data-dot]")!;
    expect(dot.className).not.toMatch(/dot-glow|ember/);
    expect(dot).not.toHaveAttribute("data-ember");
  });

  it("leaves the dot readable in both themes, with the pool beneath it at its strongest", () => {
    for (const [theme, prelude] of [
      ["dark", ':root,\n  [data-theme="dark"] {'],
      ["light", '[data-theme="light"] {'],
    ] as const) {
      const vars = { ...block(":root {\n    --violet-600"), ...block(prelude) };
      const dot = colour(vars, "--healthy");
      const pool: RGBA = [dot[0], dot[1], dot[2], 0.44 * 0.8];
      for (const g of ["--surface-1", "--canvas"]) {
        // Bare, as the spec states it (lime 4.2:1 on paper), and over the pool at its strongest: 2.8:1 on the lilac canvas (the board's own numbers; flagged to the owner).
        expect(ratio(dot, colour(vars, g)), `${theme} ${g} bare`).toBeGreaterThanOrEqual(4.2);
        expect(ratio(dot, over(pool, colour(vars, g))), `${theme} ${g} in its pool`).toBeGreaterThanOrEqual(2.8);
      }
    }
  });
});

describe("the ember opening", () => {
  it("opens only when the reader's own act cleared the queue: bloom on the empty state", () => {
    const arrival = render(<EmptyState resolved lift title="Done." />);
    expect(arrival.container.querySelector(".ember-open")).toBeNull();
    arrival.unmount();
    const earned = render(<EmptyState resolved lift bloom title="Done." />);
    const dot = earned.container.querySelector("[data-dot]")!;
    expect(dot).toHaveClass("ember", "ember-open");
    // Beside the built lift-in, in the same frame.
    expect(earned.container.querySelector("[role=status]")).toHaveClass("lift-in");
  });

  it("goes from scale 0.2 and opacity 1 to its rest over flash's duration on flash's curve, once, with no overshoot", () => {
    expect(css).toMatch(/@keyframes ember \{\s*from \{\s*opacity: 1;\s*scale: 0\.2;\s*\}\s*\}/);
    expect(glow).toMatch(/&\.ember-open::before \{\s*animation: ember var\(--duration-flash\) var\(--ease-flash\) both;/);
    expect(glow).not.toMatch(/ember-open::before \{[^}]*(infinite|cubic-bezier)/);
  });

  it("opens only where motion and colour are both on: reduced motion keeps the ember at rest, forced colours draw no light", () => {
    // The opening is inside the one media query; the rest opacity is outside it.
    expect(glow).toMatch(/@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{\s*&\.ember-open::before/);
    const outside = glow.replace(/@media[\s\S]*?\n {2}\}\n/g, "");
    expect(outside).not.toMatch(/animation:/);
    expect(outside).toMatch(/&\.ember::before \{\s*opacity: 0\.8;/);
    expect(glow).toMatch(/@media \(forced-colors: active\) \{\s*&::before \{\s*display: none;/);
  });
});

describe("bloom is gone", () => {
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((f) => {
      const p = join(dir, f);
      return statSync(p).isDirectory() ? walk(p) : /\.(tsx?|css)$/.test(f) ? [p] : [];
    });

  it("leaves no bloom keyframe, utility or class: the flat disc with an edge is not drawn anywhere", () => {
    expect(css).not.toMatch(/@keyframes bloom\b|@utility bloom\b|\.bloom\b/);
    const offenders = walk(SRC).filter(
      (p) => !p.endsWith("ember.test.tsx") && /className=(\{`[^`]*|"[^"]*)(?<=[\s"`])bloom(?=[\s"`])|querySelector\([^)]*bloom/.test(readFileSync(p, "utf8")),
    );
    expect(offenders).toEqual([]);
  });
});

// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";
import { StatusDot } from "@/components/ui/Badge";
import { CopiedSeat } from "@/components/ui/CopiedSeat";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { FoldHeading } from "@/components/ui/FoldHeading";
import { block, colour, css, over, ratio, type RGBA } from "./cssTokens";

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
    expect(glow).toMatch(/--pool-k: calc\(44% \* var\(--hour-weight\)\);/);
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
    const { container } = render(<StatusDot tone="healthy" ember ring glow />);
    const dot = container.querySelector("[data-dot]")!;
    expect(dot.className).not.toMatch(/dot-glow|ember/);
    expect(dot).not.toHaveAttribute("data-ember");
  });

  it("is worn only by a healthy dot: nothing amber, red or neutral glows (A-25 rule 5)", () => {
    for (const tone of ["warn", "danger", "neutral", "accent"] as const) {
      const { container } = render(<StatusDot tone={tone} ember glow />);
      expect(container.querySelector("[data-dot]")!.className).not.toMatch(/dot-glow|ember/);
    }
    expect(render(<StatusDot tone="healthy" glow />).container.querySelector("[data-dot]")).toHaveClass("glow-once");
  });

  it("leaves the dot readable in both themes, bare and over its pool at its strongest (the hour's weight takes the light room's down)", () => {
    for (const [theme, prelude] of [
      ["dark", ':root,\n  [data-theme="dark"] {'],
      ["light", '[data-theme="light"] {'],
    ] as const) {
      const vars = { ...block(":root {\n    --violet-600"), ...block(prelude) };
      const dot = colour(vars, "--healthy");
      const weight = Number(vars["--hour-weight"]);
      const pool: RGBA = [dot[0], dot[1], dot[2], 0.44 * weight * 0.8];
      for (const g of ["--surface-1", "--canvas"]) {
        expect(ratio(dot, colour(vars, g)), `${theme} ${g} bare`).toBeGreaterThanOrEqual(4.2);
        expect(ratio(dot, over(pool, colour(vars, g))), `${theme} ${g} in its pool`).toBeGreaterThanOrEqual(3);
      }
    }
  });

  it("is never worn by a stale card's ring, whatever a caller passed", () => {
    expect(css).toMatch(/&\[data-stale\] \[data-dot\]::before \{\s*display: none;/);
  });
});

describe("the ember opening", () => {
  it("opens only when the reader's own act cleared the queue: glow on the empty state", () => {
    const arrival = render(<EmptyState resolved lift title="Done." />);
    expect(arrival.container.querySelector(".ember-open")).toBeNull();
    arrival.unmount();
    const earned = render(<EmptyState resolved lift glow title="Done." />);
    const dot = earned.container.querySelector("[data-dot]")!;
    expect(dot).toHaveClass("ember", "ember-open");
    // Beside the built lift-in, in the same frame.
    expect(earned.container.querySelector("[role=status]")).toHaveClass("lift-in");
  });

  it("stays the same dot node when its parent re-renders, so a re-render never replays the opening", () => {
    const view = render(<EmptyState resolved lift glow title="Done." />);
    const first = view.container.querySelector("[data-dot]");
    view.rerender(<EmptyState resolved lift glow title="Done." guidance="Nothing for you to do." />);
    expect(view.container.querySelector("[data-dot]")).toBe(first);
    // What the line is first given is what it does: dropping the prop mid-opening neither cuts it short nor
    // swaps the line's animation (the same node, the same classes).
    view.rerender(<EmptyState resolved title="Done." guidance="Nothing for you to do." />);
    expect(view.container.querySelector("[data-dot]")).toBe(first);
    expect(first).toHaveClass("ember", "ember-open");
    expect(view.container.querySelector("[role=status]")).toHaveClass("lift-in");
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

describe("light is never in front of words", () => {
  it("lifts nothing itself: the dot has no isolation, and each line a once-only light sits in is the isolated one", () => {
    expect(glow).not.toMatch(/isolation/);
    const lit = [
      render(<CopiedSeat done idle="Copy" word="Copied" />),
      render(<ActionOutcome outcome={{ kind: "applied", message: "Done" }} word="Given" />),
      render(<FoldHeading open={false} onClick={() => {}} title="Done" count="1" dot="healthy" glow />),
    ];
    for (const view of lit) {
      const dot = view.container.querySelector("[data-dot].glow-once")!;
      expect(dot, "a dot that opens once").not.toBeNull();
      expect(dot).not.toHaveClass("isolate");
      expect(dot.parentElement).toHaveClass("isolate");
    }
  });
});

describe("bloom is gone", () => {
  it("leaves no bloom keyframe, utility or class in the stylesheet: the flat disc with an edge is not drawn", () => {
    expect(css).not.toMatch(/@keyframes bloom\b|@utility bloom\b|\.bloom\b|trail-answer/);
  });

  it("is one light: the trail's read dots take the same glow-once as every other dot", () => {
    expect(css.match(/@keyframes answer/g)).toHaveLength(1);
    expect(css.match(/inset: -17px/g)).toHaveLength(1);
  });
});

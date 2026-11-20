// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangeTrail, PendingTrail, TRAIL_WATCH_MS, changeTrailFor, trailSteps } from "@/components/people/ChangeTrail";
import { StatusDot } from "@/components/ui/Badge";
import { CheckTrail, type TrailCheck } from "@/components/ui/CheckTrail";
import { motionInTests } from "@/test-utils/motion";
import { SRC, css } from "./cssTokens";

// A-25 moment 5, the trail lights (owner, 20 Nov 2026: the recommendation): under a give's result, once
// Syndra has sent it, a light rests on the step the change has reached, moves on only when a read
// settles the next, and goes out when the change is home, or when the trail stops looking.

const person = vi.hoisted(() => ({ standing: "" as string, readAt: "2026-11-20T09:00:00Z", updatedAt: 0 }));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => ({
    refetch: () => Promise.resolve(),
    dataUpdatedAt: person.updatedAt,
    data: {
      projects: [{ project_id: "p1", standings: person.standing ? { operator: person.standing } : {} }],
      observation: { read_at: person.readAt, current: true, truncated: false },
    },
  }),
}));

let at = 0;
const give = (act: "give" | "revoke" = "give", sent = true) => (
  <ChangeTrail userId="u1" projectId="p1" roleKey="operator" act={act} sent={sent} at={at} />
);
const light = () => document.querySelector<HTMLElement>("[data-trail-light]");
const steps = () => screen.getAllByRole("listitem");
const current = () => steps().findIndex((li) => li.getAttribute("aria-current") === "step");
/** Where the light's box is, from the list's top: its left, top, centre and height. */
const pool = () => {
  const el = light()!;
  const [, left, top] = /^translate\((-?[\d.]+)px, (-?[\d.]+)px\)$/.exec(el.style.transform)!.map(Number);
  const height = parseFloat(el.style.height);
  return { left, top, height, centre: top + height / 2, bottom: top + height, width: parseFloat(el.style.width) };
};

// jsdom lays nothing out: each step is 36px under the last, the last one line (22.5px), the list 360 wide.
const STEP = 36;
const LIST = 2 * STEP + 22.5;
beforeEach(() => {
  at = Date.now();
  person.standing = "";
  person.readAt = "2026-11-20T09:00:00Z";
  person.updatedAt = at + 1;
  const li = (el: HTMLElement) => [...el.parentElement!.children].filter((c) => c.tagName === "LI").indexOf(el);
  vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) {
    return this.tagName === "LI" ? li(this) * STEP : 0;
  });
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockImplementation(function (this: HTMLElement) {
    return this.tagName === "OL" ? LIST : 0;
  });
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockImplementation(function (this: HTMLElement) {
    return this.tagName === "OL" ? 360 : 0;
  });
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

const open = (sentence: string): TrailCheck => ({ tone: "neutral", ring: true, sentence });
const done = (sentence: string): TrailCheck => ({ tone: "healthy", sentence });
/** A trail's block as a give's dialog draws it: 12 above, 24 each side, 4 below (`mx-6 mb-1 mt-3`). */
const Room = ({ children, margin = "12px 24px 4px" }: { children: ReactNode; margin?: string }) => (
  <div data-trail-room style={{ margin }}>
    {children}
  </div>
);
const lit = (checks: TrailCheck[], margin?: string) => (
  <Room margin={margin}>
    <CheckTrail label="On its way" light checks={checks} />
  </Room>
);

describe("the light rests on the step the change has reached", () => {
  it("is on the first open step only: one light, on that step's dot", () => {
    render(lit([done("Recorded."), open("Not sent yet."), open("Not in yet.")]));
    expect(document.querySelectorAll("[data-trail-light]")).toHaveLength(1);
    expect(current()).toBe(1);
    expect(steps().filter((li) => li.hasAttribute("aria-current"))).toHaveLength(1);
    expect(light()).toHaveAttribute("data-going");
    // On that step's dot: 11px down its row (the dot's 8px and half its 6px).
    expect(pool().centre).toBeCloseTo(STEP + 11);
  });

  it("moves only when the steps do: the same steps leave it where it is, the next step settled moves it on", () => {
    const first = [done("Recorded."), open("Not sent yet."), open("Not in yet.")];
    const view = render(lit(first));
    const resting = light()!.style.transform;
    view.rerender(lit([...first]));
    expect(light()!.style.transform).toBe(resting);
    view.rerender(lit([done("Recorded."), done("Sent."), open("Not in yet.")]));
    expect(current()).toBe(2);
    expect(pool().centre).toBeCloseTo(2 * STEP + 11);
  });

  it("appears on a give only once Syndra has sent it, by its answer or by a read", () => {
    const view = render(give("give", false));
    expect(light()).toBeNull();
    // A read that shows Zitadel took it: "Sent" settles, and the light rests on the step it reached.
    person.standing = "sent";
    view.rerender(give("give", false));
    expect(current()).toBe(2);
    expect(light()).toHaveAttribute("data-going");
    expect(pool().centre).toBeCloseTo(2 * STEP + 11);
  });

  it("goes out when the last step is read back: the change is home, and its breath is held, not reset", () => {
    motionInTests();
    const view = render(give());
    expect(light()).toHaveAttribute("data-going");
    expect(light()).toHaveAttribute("data-breathe");
    person.standing = "held";
    person.readAt = "2026-11-20T09:05:00Z";
    view.rerender(give());
    expect(current()).toBe(-1);
    // It stays where it rested, to go out there, still on its breath: the breath pauses where it is.
    expect(light()).not.toHaveAttribute("data-going");
    expect(light()).toHaveAttribute("data-breathe");
    expect(css).toMatch(/\.trail-light\[data-breathe\]:not\(\[data-going\]\)::before \{\s*animation-play-state: paused;/);
  });

  it("goes out when the trail stops looking, after TRAIL_WATCH_MS, though the step is still open", () => {
    vi.useFakeTimers();
    try {
      at = Date.now();
      person.updatedAt = at + 1;
      render(give());
      expect(light()).toHaveAttribute("data-going");
      act(() => void vi.advanceTimersByTime(TRAIL_WATCH_MS - 1));
      expect(light()).toHaveAttribute("data-going");
      act(() => void vi.advanceTimersByTime(1));
      expect(steps()[2]).toHaveTextContent("Not read back from Zitadel yet.");
      expect(light()).not.toHaveAttribute("data-going");
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("each dot fills a beat behind the light, and never empties on the way", () => {
  it("wears the light's lead on a lit trail, and not on any other", () => {
    render(lit([done("Recorded."), open("Not sent yet.")]));
    const dots = [...document.querySelectorAll("[data-dot]")];
    expect(dots).toHaveLength(2);
    for (const dot of dots) expect(dot).toHaveClass("light-lead");
    expect(/@utility light-lead \{\s*transition-delay: var\(--light-lead\);\s*\}/.test(css)).toBe(true);
  });

  it("draws a ring as an inset shadow in its tone's colour, which tints into the fill, never a border that drops away", () => {
    const { container } = render(<StatusDot tone="neutral" ring />);
    const dot = container.querySelector("[data-dot]")!;
    expect(dot).toHaveClass("dot-ring", "text-faint", "motion-tint");
    expect(dot.className).not.toMatch(/border/);
    expect(css).toMatch(/@utility dot-ring \{\s*box-shadow: inset 0 0 0 1\.5px currentColor;/);
    // The tint carries the shadow, so the ring and the fill cross over rather than one going first.
    expect(/@utility motion-tint \{\s*transition-property:[^;]*box-shadow/.test(css)).toBe(true);
    // A stale card's dots ring the same way.
    expect(css).toMatch(/&\[data-stale\] \[data-dot\] \{\s*background-color: transparent;\s*box-shadow: inset 0 0 0 1\.5px currentColor;/);
  });
});

describe("what it lights", () => {
  it("is only ever lines of ink and faint: a change's steps carry no link and nothing drawn under them", () => {
    // A violet link would sit at 4.03:1 on the light's centre in the dark room (contrast.test.ts).
    for (const act of ["give", "revoke"] as const) {
      for (const sent of [true, false]) {
        for (const standing of [undefined, "sent", "held", "not_held"] as const) {
          for (const step of trailSteps(act, sent, standing, { readAt: "2026-11-20T09:00:00Z" })) {
            expect(step.link).toBeUndefined();
            expect(step.children).toBeUndefined();
          }
        }
      }
    }
  });
});

describe("a route's current step is the route's, whether or not it is lit", () => {
  it("marks the step a route has reached, lit or not, and nothing on a set of checks", () => {
    render(<CheckTrail label="On its way" checks={[done("Recorded."), open("Not sent yet.")]} />);
    expect(current()).toBe(1);
    expect(light()).toBeNull();
  });
});

describe("where it never is", () => {
  it("has no trail at all on a refusal or a failure", () => {
    expect(changeTrailFor("failed", undefined)).toBeUndefined();
    expect(changeTrailFor("pending", "refused:held")).toBeUndefined();
  });

  it("is not on a removal's trail: nothing that takes access away is lit", () => {
    render(give("revoke"));
    expect(light()).toBeNull();
    expect(current()).toBe(2);
    expect(document.querySelector(".light-lead")).toBeNull();
  });

  it("is not on a give that waits for someone to press Send: Syndra is not moving it", () => {
    for (const standing of ["", "waiting"]) {
      person.standing = standing;
      const view = render(give("give", false));
      expect(steps()[1]).toHaveTextContent("Not sent to Zitadel yet.");
      expect(light()).toBeNull();
      view.unmount();
    }
  });

  it("is not on the route before the answer: nothing has been read", () => {
    render(<PendingTrail act="give" />);
    expect(light()).toBeNull();
  });

  it("is not on a check's trail (debug mode, the preview): only a change's trail passes a light", () => {
    render(<CheckTrail compact checks={[done("Signed in."), open("Last answer.")]} />);
    expect(light()).toBeNull();
    expect(current()).toBe(-1);
    // And no one else passes one, as a value or bare: the only light on a trail is the change's own.
    for (const file of ["components/apps/DebugMode.tsx", "components/apps/ClaimPreview.tsx", "components/people/RemovalDialog.tsx"]) {
      const source = readFileSync(join(SRC, file), "utf8");
      expect(source, file).not.toMatch(/<(CheckTrail|ChangeTrail)\b[^>]*\slight(?=[\s=/>])/);
    }
  });
});

describe("its motion", () => {
  it("travels and breathes when motion is on", () => {
    motionInTests();
    render(lit([done("Recorded."), open("Not sent yet.")]));
    expect(light()).toHaveClass("trail-light-glide");
    expect(light()).toHaveAttribute("data-breathe");
  });

  it("neither travels nor breathes when still (reduced motion): it rests on the step", () => {
    motionInTests(false);
    render(lit([done("Recorded."), open("Not sent yet.")]));
    expect(light()).toHaveAttribute("data-going");
    expect(light()).not.toHaveClass("trail-light-glide");
    expect(light()).not.toHaveAttribute("data-breathe");
  });

  it("is a pool of the accent at 26% of the room's weight, travelling on tint's curve over flash and going out on flash's", () => {
    const rule = /\n  \.trail-light \{([\s\S]*?)\n  \}\n/.exec(css)?.[1] ?? "";
    expect(rule).toMatch(/--pool-k: calc\(26% \* var\(--hour-weight\)\);/);
    expect(rule).toMatch(/transform var\(--trail-travel, 0s\) var\(--ease-tint\),\s*opacity var\(--duration-flash\) var\(--ease-flash\);/);
    expect(rule).toMatch(/background: radial-gradient\(closest-side, var\(--pool-stops\)\);/);
    // It breathes only for a visitor who has not asked for less (lamp.test.tsx holds every glow to that).
    expect(css).toMatch(
      /@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{\s*\.trail-light\[data-breathe\]::before \{\s*animation: breathe var\(--duration-breathe\) var\(--ease-breathe\) infinite alternate;/,
    );
    expect(css).toMatch(/\.trail-light-glide \{\s*--trail-travel: var\(--duration-flash\);/);
  });

  it("is not drawn under forced colours", () => {
    expect(css).toMatch(/@media \(forced-colors: active\) \{\s*\.trail-light \{\s*display: none;/);
  });

  it("is not one of the list's steps to the glide indicator's observer", () => {
    const observed: Element[] = [];
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe(el: Element) {
          observed.push(el);
        }
        disconnect() {}
      },
    );
    render(lit([done("Recorded."), open("Not sent yet.")]));
    expect(light()).toHaveAttribute("data-indicator");
    expect(observed.length).toBeGreaterThan(0);
    expect(observed).not.toContain(light());
  });
});

describe("no box cuts it (rule 1): sized by its own room", () => {
  it("is spent inside its block's own margins, whatever is around it, on every step", () => {
    // `mx-6 mb-1 mt-3`: from 12 above the list to 4 below it, and 4px short of the 24 at its side.
    for (const [name, checks, step] of [
      ["the first step", [open("Not recorded yet."), open("Not sent yet."), open("Not in yet.")], 0],
      ["the second step", [done("Recorded."), open("Not sent yet."), open("Not in yet.")], 1],
      ["the last step", [done("Recorded."), done("Sent."), open("Not read back yet.")], 2],
    ] as const) {
      const view = render(lit([...checks]));
      const p = pool();
      expect(p.centre, name).toBeCloseTo(step * STEP + 11);
      expect(p.left, name).toBe(-20);
      expect(p.top, name).toBeGreaterThanOrEqual(-12);
      expect(p.bottom, name).toBeLessThanOrEqual(LIST + 4);
      expect(p.width, name).toBeLessThanOrEqual(360 + 20);
      view.unmount();
    }
    // The last step has 11.5px of its own row and 4 of margin under its dot: 31 tall, all it has.
    const view = render(lit([done("Recorded."), done("Sent."), open("Not read back yet.")]));
    expect(pool().height).toBeCloseTo(31);
    view.unmount();
  });

  it("takes its side from the room's own margin, never a number of the caller's", () => {
    for (const [margin, left] of [
      ["12px 24px 4px", -20],
      ["12px 16px 4px", -12],
      ["12px 2px 4px", 0],
    ] as const) {
      const view = render(lit([done("Recorded."), open("Not sent yet."), open("Not in yet.")], margin));
      expect(pool().left, margin).toBe(left);
      expect(pool().width, margin).toBeLessThanOrEqual(360 - left);
      view.unmount();
    }
  });
});

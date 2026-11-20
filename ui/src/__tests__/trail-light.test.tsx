// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangeTrail, PendingTrail, TRAIL_WATCH_MS, changeTrailFor, trailSteps } from "@/components/people/ChangeTrail";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { CheckTrail, type TrailCheck } from "@/components/ui/CheckTrail";
import { BREATHE_MS, FLASH_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";
import { SRC, css } from "./cssTokens";

// A-25 moment 5, the trail lights (owner, 20 Nov 2026: the recommendation; then, on staging, "could barely
// see it": the walk home, and the trail inside its result). Under a give's result, once Syndra has sent it,
// a light appears on the first step and walks down the steps already seen to the step the change has
// reached, waits there breathing, moves on when a read settles the next, and when the change is home rests
// one breath on the last step and goes out; it also goes out when the trail stops looking.

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

/** The result's card the trail sits in, as the dialog draws it: the card is the light's room. */
const Card = ({ children }: { children: ReactNode }) => <div data-trail-room>{children}</div>;
let at = 0;
const give = (act: "give" | "revoke" = "give", sent = true) => (
  <Card>
    <ChangeTrail userId="u1" projectId="p1" roleKey="operator" act={act} sent={sent} at={at} />
  </Card>
);
const lit = (checks: TrailCheck[]) => (
  <Card>
    <CheckTrail label="On its way" light checks={checks} />
  </Card>
);
const light = () => document.querySelector<HTMLElement>("[data-trail-light]");
const steps = () => screen.getAllByRole("listitem");
const current = () => steps().findIndex((li) => li.getAttribute("aria-current") === "step");
const dots = () => [...document.querySelectorAll("[data-dot]")];
/** Where the light's box is, from the list's top left. */
const pool = () => {
  const el = light()!;
  const [, left, top] = /^translate\((-?[\d.]+)px, (-?[\d.]+)px\)$/.exec(el.style.transform)!.map(Number);
  const height = parseFloat(el.style.height);
  const width = parseFloat(el.style.width);
  return { left, top, height, width, centre: top + height / 2, bottom: top + height, right: left + width, step: Number(el.dataset.step) };
};
const tick = (ms: number) => act(() => void vi.advanceTimersByTime(ms));

// jsdom lays nothing out, so the dialog's give is laid out here: the card's inner box is 470 by 200, with
// its 1px border at 0,0; the list sits 20px in (the card's padding) and 59px down (the word and sentence
// above it); its steps are 36px apart, so each dot's centre is 11, 47 and 83 down the list.
const STEP = 36;
const CARD = { width: 470, height: 200, border: 1 };
const LIST = { left: 21, top: 60 };
const INNER = { left: -20, top: -59, right: -20 + CARD.width, bottom: -59 + CARD.height };
const rect = (left: number, top: number, width = 0, height = 0) =>
  ({ left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON() {} }) as DOMRect;
beforeEach(() => {
  at = Date.now();
  person.standing = "";
  person.readAt = "2026-11-20T09:00:00Z";
  person.updatedAt = at + 1;
  const index = (el: HTMLElement) => [...el.parentElement!.children].filter((c) => c.tagName === "LI").indexOf(el);
  vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) {
    return this.tagName === "LI" ? index(this) * STEP : 0;
  });
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    if (this.matches("[data-trail-room]")) return rect(0, 0, CARD.width + 2, CARD.height + 2);
    if (this.matches("ol")) return rect(LIST.left, LIST.top, 430, 2 * STEP + 22.5);
    return rect(0, 0);
  });
  const of = (el: Element, value: number) => (el.matches("[data-trail-room]") ? value : 0);
  vi.spyOn(Element.prototype, "clientTop", "get").mockImplementation(function (this: Element) {
    return of(this, CARD.border);
  });
  vi.spyOn(Element.prototype, "clientLeft", "get").mockImplementation(function (this: Element) {
    return of(this, CARD.border);
  });
  vi.spyOn(Element.prototype, "clientWidth", "get").mockImplementation(function (this: Element) {
    return of(this, CARD.width);
  });
  vi.spyOn(Element.prototype, "clientHeight", "get").mockImplementation(function (this: Element) {
    return of(this, CARD.height);
  });
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

const open = (sentence: string): TrailCheck => ({ tone: "neutral", ring: true, sentence });
const done = (sentence: string): TrailCheck => ({ tone: "healthy", sentence });

describe("the walk home", () => {
  it("appears on the first step and walks down each step already seen, in order, one a flash, to the open step", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    const visited = [pool().step];
    expect(pool().centre).toBeCloseTo(11);
    for (let i = 0; i < 4; i++) {
      tick(FLASH_MS);
      if (pool().step !== visited.at(-1)) visited.push(pool().step);
    }
    expect(visited).toEqual([0, 1, 2]);
    expect(pool().centre).toBeCloseTo(2 * STEP + 11);
    // The open step: it waits there, breathing, and does not go out.
    tick(BREATHE_MS * 3);
    expect(pool().step).toBe(2);
    expect(light()).toHaveAttribute("data-going");
    expect(light()).toHaveAttribute("data-breathe");
    expect(light()).toHaveClass("trail-light-glide");
  });

  it("appears where it is: placed first, it travels only after, never sliding in from the list's corner", async () => {
    motionInTests();
    const classes: string[] = [];
    const watch = new MutationObserver((records) => {
      for (const r of records) if ((r.target as Element).matches?.("[data-trail-light]")) classes.push(r.oldValue ?? "");
    });
    watch.observe(document.body, { subtree: true, attributes: true, attributeFilter: ["class"], attributeOldValue: true });
    render(give());
    await Promise.resolve();
    watch.disconnect();
    // It was drawn without the travel first; the travel came after.
    expect(classes.length).toBeGreaterThan(0);
    expect(classes[0]).not.toMatch(/trail-light-glide/);
    expect(light()).toHaveClass("trail-light-glide");
  });

  it("walks over what is true: every dot it passes is already filled, and stays filled, at every beat", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    for (let t = 0; t <= 3 * FLASH_MS; t += FLASH_MS / 3) {
      const [recorded, sent, there] = dots();
      expect(recorded, `at ${t}ms`).not.toHaveAttribute("data-ring");
      expect(sent, `at ${t}ms`).not.toHaveAttribute("data-ring");
      expect(there, `at ${t}ms`).toHaveAttribute("data-ring");
      tick(FLASH_MS / 3);
    }
  });

  it("runs once: a re-render, or a step settling after it, moves the light at once and never walks again", () => {
    motionInTests();
    vi.useFakeTimers();
    const checks = [done("Recorded."), open("Not sent yet."), open("Not in yet.")];
    const view = render(lit(checks));
    tick(FLASH_MS * 3);
    expect(pool().step).toBe(1);
    view.rerender(lit([...checks]));
    expect(pool().step).toBe(1);
    // A step settles while it waits: it moves on at once, the dot filling a beat behind it.
    view.rerender(lit([done("Recorded."), done("Sent."), open("Not in yet.")]));
    expect(pool().step).toBe(2);
    expect(dots()[1]).toHaveClass("light-lead");
  });

  it("rests one breath on the last step once the change is home, and goes out", () => {
    motionInTests();
    vi.useFakeTimers();
    render(lit([done("Recorded."), done("Sent."), done("In.")]));
    tick(FLASH_MS);
    tick(FLASH_MS);
    expect(pool().step).toBe(2);
    // Arriving takes the flash; then one breath.
    tick(FLASH_MS + BREATHE_MS - 1);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(light()).not.toHaveAttribute("data-going");
    // Its breath is held where it is while it fades, never reset to full.
    expect(light()).toHaveAttribute("data-breathe");
    expect(css).toMatch(/\.trail-light\[data-breathe\]:not\(\[data-going\]\)::before \{\s*animation-play-state: paused;/);
  });

  it("does not walk when still (reduced motion): it rests on the last settled step for one breath, then goes", () => {
    motionInTests(false);
    vi.useFakeTimers();
    render(lit([done("Recorded."), done("Sent."), done("In.")]));
    expect(pool().step).toBe(2);
    expect(light()).not.toHaveClass("trail-light-glide");
    expect(light()).not.toHaveAttribute("data-breathe");
    tick(BREATHE_MS - 1);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(light()).not.toHaveAttribute("data-going");
  });

  it("rests on the open step from the first frame when still, without breath", () => {
    motionInTests(false);
    render(give());
    expect(pool().step).toBe(2);
    expect(light()).toHaveAttribute("data-going");
    expect(light()).not.toHaveAttribute("data-breathe");
  });

  it("goes out when the trail stops looking, after TRAIL_WATCH_MS, though the step is still open", () => {
    vi.useFakeTimers();
    at = Date.now();
    person.updatedAt = at + 1;
    render(give());
    expect(light()).toHaveAttribute("data-going");
    tick(TRAIL_WATCH_MS - 1);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(steps()[2]).toHaveTextContent("Not read back from Zitadel yet.");
    expect(light()).not.toHaveAttribute("data-going");
  });

  it("travels on tint's curve over flash, a pool of the accent at 26% of the room's weight, going out on flash's", () => {
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
});

describe("a step settling while it waits", () => {
  it("appears on a give only once Syndra has sent it, by its answer or by a read", () => {
    const view = render(give("give", false));
    expect(light()).toBeNull();
    person.standing = "sent";
    view.rerender(give("give", false));
    expect(current()).toBe(2);
    expect(light()).toHaveAttribute("data-going");
  });

  it("wears the light's lead on every dot of a lit trail: only a dot that changes ever uses it", () => {
    render(lit([done("Recorded."), open("Not sent yet.")]));
    expect(dots()).toHaveLength(2);
    for (const dot of dots()) expect(dot).toHaveClass("light-lead");
    expect(/@utility light-lead \{\s*transition-delay: var\(--light-lead\);\s*\}/.test(css)).toBe(true);
  });

  it("draws a ring as an inset shadow in its tone's colour, which tints into the fill, never a border that drops away", () => {
    const { container } = render(<StatusDot tone="neutral" ring />);
    const dot = container.querySelector("[data-dot]")!;
    expect(dot).toHaveClass("dot-ring", "text-faint", "motion-tint");
    expect(dot.className).not.toMatch(/border/);
    expect(css).toMatch(/@utility dot-ring \{\s*box-shadow: inset 0 0 0 1\.5px currentColor;/);
    // Forced colours drop every shadow: there the ring is a border in the system's colour.
    expect(css).toMatch(
      /@utility dot-ring \{\s*box-shadow: inset 0 0 0 1\.5px currentColor;[^}]*@media \(forced-colors: active\) \{\s*border: 1\.5px solid currentColor;/,
    );
    // The tint carries the shadow, so the ring and the fill cross over rather than one going first.
    expect(/@utility motion-tint \{\s*transition-property:[^;]*box-shadow/.test(css)).toBe(true);
    // A stale card's dots ring the same way: the one ring, applied, so its forced-colours border comes with it.
    expect(css).toMatch(/&\[data-stale\] \[data-dot\] \{\s*background-color: transparent;\s*@apply dot-ring;\s*\}/);
  });
});

describe("what it lights", () => {
  it("is only ever lines of ink and faint: a change's steps carry no link and nothing drawn under them", () => {
    // A violet link would sit under 4.5:1 on the light's centre in the dark room (contrast.test.ts).
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

  it("marks nothing before the answer: a route with no step seen has reached none", () => {
    render(<PendingTrail act="give" />);
    expect(steps()).toHaveLength(3);
    expect(current()).toBe(-1);
  });

  it("is the step reached, whatever step the light is walking", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    expect(pool().step).toBe(0);
    expect(current()).toBe(2);
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

describe("its place: inside the result's card (rule 1)", () => {
  it("sits in the outcome's card, which is its room, under what happened and above what to do next", () => {
    const { container } = render(
      <ActionOutcome
        outcome={{ kind: "applied", message: "Becky Lynch can use Operator on their own now" }}
        word="Given"
        trail={<div data-testid="trail" />}
      />,
    );
    const card = container.querySelector("[data-trail-room]")!;
    expect(card).toHaveAttribute("role", "status");
    const parts = [...card.children];
    const trail = parts.findIndex((p) => p.matches("[data-testid=trail]"));
    expect(trail).toBeGreaterThan(parts.findIndex((p) => p.textContent?.includes("Becky Lynch can use Operator")));
    expect(parts.slice(trail + 1).some((p) => /Nothing for you to do/.test(p.textContent ?? ""))).toBe(true);
  });

  it("is the card's only when it holds a trail", () => {
    const { container } = render(<ActionOutcome outcome={{ kind: "applied", message: "Done" }} word="Given" />);
    expect(container.querySelector("[data-trail-room]")).toBeNull();
  });

  it("gets its whole 280 by 96 on every step it walks, spent inside the card's inner box", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    for (const step of [0, 1, 2]) {
      const p = pool();
      expect(p.step).toBe(step);
      expect(p.centre, `step ${step}`).toBeCloseTo(step * STEP + 11);
      expect([p.width, p.height], `step ${step}`).toEqual([280, 96]);
      expect(p.left, `step ${step}`).toBeGreaterThanOrEqual(INNER.left);
      expect(p.top, `step ${step}`).toBeGreaterThanOrEqual(INNER.top);
      expect(p.right, `step ${step}`).toBeLessThanOrEqual(INNER.right);
      expect(p.bottom, `step ${step}`).toBeLessThanOrEqual(INNER.bottom);
      tick(FLASH_MS);
    }
  });

  it("is never larger than the card it sits in: a shallow card keeps it inside its edge", () => {
    vi.spyOn(Element.prototype, "clientHeight", "get").mockImplementation(function (this: Element) {
      return this.matches("[data-trail-room]") ? 59 + 2 * STEP + 22.5 + 16 : 0;
    });
    render(give());
    const p = pool();
    expect(p.step).toBe(2);
    expect(p.bottom).toBeLessThanOrEqual(2 * STEP + 22.5 + 16);
    expect(p.height).toBeLessThan(96);
  });
});

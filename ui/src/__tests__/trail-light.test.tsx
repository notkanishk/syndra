// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangeTrail, PendingTrail, TRAIL_WATCH_MS, changeTrailFor, trailSteps } from "@/components/people/ChangeTrail";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { BEAT_MS, CheckTrail, type TrailCheck } from "@/components/ui/CheckTrail";
import { ARRIVE_MS, BREATHE_MS, FLASH_MS, LIGHT_LEAD_MS, SETTLE_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";
import { SRC, css } from "./cssTokens";

// A-25 moment 5, the trail lights: the lamp carried home (owner, 20 Nov 2026: "Carried", the recommendation,
// after "the bloom moves, the dots do not, it is not organic"). Under a give's result, once Syndra had sent
// it, a lamp centred on the dots is carried down the steps already seen in one motion, the hairline lit where
// it is, each read dot's own glow opening once as it is reached. It waits and breathes on the step reached,
// or goes out a beat after the last; a read that settles the step sends it on.

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
const path = () => document.querySelector<HTMLElement>("[data-trail-path]");
const steps = () => screen.getAllByRole("listitem");
const current = () => steps().findIndex((li) => li.getAttribute("aria-current") === "step");
const dots = () => [...document.querySelectorAll("li [data-dot]")];
const px = (el: HTMLElement, name: string) => parseFloat(el.style.getPropertyValue(name));
/** Where a stack of moves has carried what is inside it: the sum of the moves that have set out. */
const carried = (root: HTMLElement) =>
  [...root.querySelectorAll<HTMLElement>(".trail-move")].reduce((sum, move) => sum + (parseFloat(/translateY\((-?[\d.]+)px\)/.exec(move.style.transform)?.[1] ?? "0") || 0), 0);
/** The lamp's centre, from the list's top: the first dot, and the sum of its moves. */
const lampAt = () => px(light()!, "--trail-y") + carried(light()!);
const moves = () => Number(light()!.dataset.moves);
const answered = () => dots().map((d) => d.classList.contains("glow-once"));
/**
 * Time passing, a millisecond at a time by default: React answers a timer as it fires in a browser, so each
 * timer the answer sets is set at the time it really is, not at the end of a long jump.
 */
const tick = (ms: number, step = 1) => {
  for (let t = 0; t < ms; t += step) act(() => void vi.advanceTimersByTime(Math.min(step, ms - t)));
};

// jsdom lays nothing out, so the dialog's give is laid out here: the card's inner box is 470 by 200, with
// its 1px border at 0,0; the list sits 20px in (the card's padding) and 59px down (the word and sentence
// above it); its steps are 36px apart, so each dot's centre is 11, 47 and 83 down the list, 7 across.
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
    if (this.matches("[data-trail]")) return rect(LIST.left, LIST.top, 430, 2 * STEP + 22.5);
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

describe("the lamp: one shape, on the dots", () => {
  it("is centred on the first dot, on the line, and keeps one shape for the whole walk", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    const lamp = light()!;
    expect(px(lamp, "--trail-x")).toBe(7);
    expect(px(lamp, "--trail-y")).toBe(11);
    const shape = ["--reach-left", "--reach-right", "--reach-up", "--reach-down"].map((v) => lamp.style.getPropertyValue(v));
    // Over the sentence 240, up and down 56 at most, on the left the card's room less the 4px kept.
    expect(shape).toEqual(["23px", "240px", "56px", "54px"]);
    for (let t = 0; t < ARRIVE_MS + 3 * BEAT_MS; t += BEAT_MS / 2) {
      tick(BEAT_MS / 2);
      expect(["--reach-left", "--reach-right", "--reach-up", "--reach-down"].map((v) => light()!.style.getPropertyValue(v))).toEqual(shape);
    }
    // The line is the dots' own: from the first dot's centre to the last, at their x.
    expect([px(path()!, "--trail-x"), px(path()!, "--trail-y"), px(path()!, "--trail-length")]).toEqual([7, 11, 2 * STEP]);
  });

  it("is spent inside the card's inner box on every step it is carried to", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    const lamp = light()!;
    const [left, right, up, down] = ["--reach-left", "--reach-right", "--reach-up", "--reach-down"].map((v) => px(lamp, v));
    for (const step of [0, 1, 2]) {
      const y = 11 + step * STEP;
      expect(7 - left, `step ${step}`).toBeGreaterThanOrEqual(INNER.left + 4);
      expect(7 + right, `step ${step}`).toBeLessThanOrEqual(INNER.right - 4);
      expect(y - up, `step ${step}`).toBeGreaterThanOrEqual(INNER.top + 4);
      expect(y + down, `step ${step}`).toBeLessThanOrEqual(INNER.bottom - 4);
    }
  });

  it("is never larger than the card it sits in: a shallow card shortens its reach, never its place", () => {
    vi.spyOn(Element.prototype, "clientHeight", "get").mockImplementation(function (this: Element) {
      return this.matches("[data-trail-room]") ? 59 + 2 * STEP + 22.5 + 16 : 0;
    });
    render(give());
    const down = px(light()!, "--reach-down");
    expect(down).toBeLessThan(56);
    expect(11 + 2 * STEP + down).toBeLessThanOrEqual(2 * STEP + 22.5 + 16 - 4);
  });
});

describe("carried home", () => {
  it("swells in on the first dot, then sets out for each next step a beat apart, each move longer than a beat", () => {
    expect(BEAT_MS).toBe(FLASH_MS - SETTLE_MS);
    expect(BEAT_MS).toBeLessThan(FLASH_MS);
    motionInTests();
    vi.useFakeTimers();
    render(give());
    expect(light()).toHaveAttribute("data-going");
    expect(light()).toHaveAttribute("data-carried");
    expect([moves(), lampAt()]).toEqual([0, 11]);
    tick(ARRIVE_MS - 1);
    expect(moves()).toBe(0);
    tick(1);
    expect([moves(), lampAt()]).toEqual([1, 11 + STEP]);
    // The next sets out a beat later, while the first is still on its way (a flash long): they overlap.
    tick(BEAT_MS - 1);
    expect(moves()).toBe(1);
    tick(1);
    expect([moves(), lampAt()]).toEqual([2, 11 + 2 * STEP]);
    // The line is carried by the same moves.
    expect(carried(path()!)).toBe(2 * STEP);
    tick(BEAT_MS * 3);
    expect(moves()).toBe(2);
    expect(css).toMatch(/\.trail-move \{[^}]*\[data-carried\] & \{\s*transition: transform var\(--duration-flash\) var\(--ease-tint\);/);
  });

  it("walks over what is true: every read dot keeps its fill on every beat, and a ring stays a ring", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    for (let t = 0; t <= ARRIVE_MS + 3 * BEAT_MS; t += LIGHT_LEAD_MS) {
      const [recorded, sent, there] = dots();
      for (const dot of [recorded, sent]) {
        expect(dot, `at ${t}ms`).toHaveClass("bg-healthy");
        expect(dot, `at ${t}ms`).not.toHaveAttribute("data-ring");
      }
      expect(there, `at ${t}ms`).toHaveAttribute("data-ring");
      tick(LIGHT_LEAD_MS);
    }
  });

  it("each read dot answers once, a beat behind the lamp reaching it; a ring never answers", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    expect(answered()).toEqual([false, false, false]);
    tick(ARRIVE_MS + LIGHT_LEAD_MS - 1);
    expect(answered()).toEqual([false, false, false]);
    tick(1);
    expect(answered()).toEqual([true, false, false]);
    tick(BEAT_MS - 1);
    expect(answered()).toEqual([true, false, false]);
    tick(1);
    expect(answered()).toEqual([true, true, false]);
    tick(BEAT_MS * 4);
    expect(answered()).toEqual([true, true, false]);
    // Its glow is the ember's light, opening once: a keyframe set of its own, not `bloom`'s disc.
    expect(css).toMatch(/@keyframes answer \{\s*from \{\s*opacity: 0;\s*scale: 0\.3;\s*\}\s*30% \{\s*opacity: 1;\s*\}\s*to \{\s*opacity: 0;\s*scale: 1;/);
    expect(css).toMatch(/&\.glow-once::before \{\s*animation: answer var\(--duration-flash\) var\(--ease-flash\) both;/);
  });

  it("does not breathe while it walks; it breathes only once it has landed on a step not yet read", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    for (let t = 0; t < ARRIVE_MS + 2 * BEAT_MS + FLASH_MS; t += 100) {
      expect(light(), `at ${t}ms`).not.toHaveAttribute("data-breathing");
      tick(100);
    }
    tick(100);
    expect(light()).toHaveAttribute("data-breathing");
  });

  it("when the read lands, it stops breathing, the dot answers a beat behind, and the lamp goes out a beat later", () => {
    motionInTests();
    vi.useFakeTimers();
    const view = render(give());
    tick(ARRIVE_MS + 2 * BEAT_MS + FLASH_MS + 100);
    expect(light()).toHaveAttribute("data-breathing");
    person.standing = "held";
    person.readAt = "2026-11-20T09:05:00Z";
    view.rerender(give());
    expect(light()).not.toHaveAttribute("data-breathing");
    expect(answered()[2]).toBe(false);
    tick(LIGHT_LEAD_MS);
    expect(answered()[2]).toBe(true);
    expect(light()).toHaveAttribute("data-going");
    tick(BEAT_MS - LIGHT_LEAD_MS);
    expect(light()).not.toHaveAttribute("data-going");
    expect(path()).not.toHaveAttribute("data-going");
  });

  it("goes out a beat after the last step once the change is home, and runs once", () => {
    motionInTests();
    vi.useFakeTimers();
    const checks = [done("Recorded."), done("Sent."), done("In.")];
    const view = render(lit(checks));
    tick(ARRIVE_MS + 2 * BEAT_MS);
    expect(moves()).toBe(2);
    expect(answered()).toEqual([true, true, false]);
    tick(LIGHT_LEAD_MS);
    expect(answered()).toEqual([true, true, true]);
    tick(BEAT_MS - LIGHT_LEAD_MS - 1);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(light()).not.toHaveAttribute("data-going");
    // Once: a re-render never sets it walking again.
    view.rerender(lit([...checks]));
    tick(ARRIVE_MS + 3 * BEAT_MS);
    expect(moves()).toBe(2);
    expect(light()).not.toHaveAttribute("data-going");
  });

  it("is still under reduced motion: one place, no glow, no breath, and home it goes one breath later", () => {
    motionInTests(false);
    vi.useFakeTimers();
    render(lit([done("Recorded."), done("Sent."), done("In.")]));
    expect(light()).not.toHaveAttribute("data-carried");
    expect([moves(), lampAt()]).toEqual([2, 11 + 2 * STEP]);
    tick(LIGHT_LEAD_MS * 2);
    expect(answered()).toEqual([false, false, false]);
    expect(light()).not.toHaveAttribute("data-breathing");
    tick(BREATHE_MS - LIGHT_LEAD_MS * 2 - 1);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(light()).not.toHaveAttribute("data-going");
  });

  it("rests on the open step from the first frame when still, without breath", () => {
    motionInTests(false);
    vi.useFakeTimers();
    render(give());
    expect(moves()).toBe(2);
    tick(FLASH_MS * 3);
    expect(light()).toHaveAttribute("data-going");
    expect(light()).not.toHaveAttribute("data-breathing");
  });

  it("goes out when the trail stops looking, after TRAIL_WATCH_MS, though the step is still open", () => {
    vi.useFakeTimers();
    at = Date.now();
    person.updatedAt = at + 1;
    render(give());
    expect(light()).toHaveAttribute("data-going");
    tick(TRAIL_WATCH_MS - 1, 999);
    expect(light()).toHaveAttribute("data-going");
    tick(1);
    expect(steps()[2]).toHaveTextContent("Not read back from Zitadel yet.");
    expect(light()).not.toHaveAttribute("data-going");
  });

  it("is a pool of the accent at 26% of the room's weight in four quarters, swelling in and going out on tokens", () => {
    const rule = /\n  \.trail-light \{([\s\S]*?)\n  \}\n/.exec(css)?.[1] ?? "";
    expect(rule).toMatch(/--pool-k: calc\(26% \* var\(--hour-weight\)\);/);
    expect(rule).toMatch(/transition: opacity var\(--duration-flash\) var\(--ease-flash\);/);
    expect(rule).toMatch(/&\[data-going\] \{\s*opacity: 1;\s*transition: opacity var\(--duration-arrive\) var\(--ease-settle\);/);
    expect(css).toMatch(/background: radial-gradient\(ellipse farthest-side at var\(--at\), var\(--pool-stops\)\);/);
    // Four quarters round the dot, each its own reach and its own far corner.
    for (const [q, x, y, at] of [
      ["tl", "right", "bottom", "100% 100%"],
      ["tr", "left", "bottom", "0% 100%"],
      ["bl", "right", "top", "100% 0%"],
      ["br", "left", "top", "0% 0%"],
    ]) {
      const quarter = new RegExp(`& > \\[data-quarter="${q}"\\] \\{([^}]*)\\}`).exec(css)?.[1] ?? "";
      expect(quarter, q).toMatch(new RegExp(`${x}: 0;\\s*${y}: 0;`));
      expect(quarter, q).toMatch(/width: var\(--reach-(left|right)\);\s*height: var\(--reach-(up|down)\);/);
      expect(quarter, q).toContain(`--at: ${at};`);
    }
  });

  it("swells in from 86% on a real trail, where the moves wrap the pool: by its state, not by where it sits", () => {
    motionInTests();
    render(give());
    const pool = light()!.querySelector(".trail-pool")!;
    // Three steps: two moves, and the pool inside both.
    expect(light()!.querySelectorAll(".trail-move")).toHaveLength(2);
    expect(pool.closest(".trail-move")).not.toBeNull();
    expect(pool.parentElement).not.toBe(light());
    // So the rule that swells it reads its state through any depth: a descendant, never a child.
    const rule = /\.trail-pool \{([\s\S]*?)\n  \}\n/.exec(css)?.[1] ?? "";
    expect(rule).toMatch(/^\s*position: absolute;[\s\S]*?\n    scale: 0\.86;/);
    expect(rule).toMatch(/\n    \[data-going\] & \{\s*scale: 1;\s*@starting-style \{\s*scale: 0\.86;/);
    expect(rule).not.toMatch(/\[data-going\] > &/);
    expect(rule).toMatch(/transition:\s*scale var\(--duration-arrive\) var\(--ease-settle\)/);
  });

  it("is beside its list, never in it: the list holds only its steps", () => {
    render(give());
    const list = screen.getByRole("list");
    expect([...list.children].every((child) => child.tagName === "LI")).toBe(true);
    const box = list.parentElement!;
    expect(box).toHaveAttribute("data-trail");
    expect(box).toHaveClass("relative");
    // The lamp and its line come first, so the dots, later in the box, stand in front of the line.
    expect([...box.children].map((c) => (c.matches("[data-trail-light]") ? "lamp" : c.matches("[data-trail-path]") ? "line" : c.tagName.toLowerCase()))).toEqual([
      "lamp",
      "line",
      "ol",
    ]);
  });

  it("walks a late read one beat at a time: a read that settles two steps never jumps them", () => {
    motionInTests();
    vi.useFakeTimers();
    const view = render(lit([done("Recorded."), open("Not sent yet."), open("Not in yet."), open("Not in the add-on yet.")]));
    tick(ARRIVE_MS + BEAT_MS + FLASH_MS);
    expect(moves()).toBe(1);
    view.rerender(lit([done("Recorded."), done("Sent."), done("In."), open("Not in the add-on yet.")]));
    // It sets out at once, and the next move a beat later, never both together.
    tick(1);
    expect(moves()).toBe(2);
    tick(BEAT_MS - 2);
    expect(moves()).toBe(2);
    tick(1);
    expect(moves()).toBe(3);
    // Each dot answers with the lamp: the one it was on at once, the next a beat later.
    expect(answered()).toEqual([true, true, false, false]);
    tick(LIGHT_LEAD_MS);
    expect(answered()).toEqual([true, true, true, false]);
  });

  it("is not drawn under forced colours: no lamp, no line, no glow", () => {
    expect(css).toMatch(/@media \(forced-colors: active\) \{\s*\.trail-light,\s*\.trail-path \{\s*display: none;/);
    expect(css).toMatch(/@media \(forced-colors: active\) \{\s*&::before \{\s*display: none;/);
  });
});

describe("a step settling while it waits", () => {
  it("is for the act's own going out: a give queued for Send and sent later settles its step, and never walks", () => {
    const view = render(give("give", false));
    expect(light()).toBeNull();
    person.standing = "sent";
    view.rerender(give("give", false));
    expect(steps()[1]).toHaveTextContent("Sent to Zitadel.");
    expect(current()).toBe(2);
    expect(light()).toBeNull();
  });

  it("keeps a step it has seen: once sent, a later read that has lost sight of it never takes it back", () => {
    const view = render(give("give", false));
    person.standing = "sent";
    view.rerender(give("give", false));
    expect(steps()[1]).toHaveTextContent("Sent to Zitadel.");
    person.standing = "";
    view.rerender(give("give", false));
    expect(steps()[1]).toHaveTextContent("Sent to Zitadel.");
    expect(current()).toBe(2);
  });

  it("is read where it is: inside its result's status, a step settling does not read the whole card again", () => {
    render(give());
    expect(screen.getByRole("list").closest("[aria-live]")).toHaveAttribute("aria-live", "off");
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

  it("is the step reached, whatever step the lamp is carried to", () => {
    motionInTests();
    vi.useFakeTimers();
    render(give());
    expect(moves()).toBe(0);
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
    expect(container.firstElementChild).not.toHaveClass("isolate");
  });

  it("lies under every line in the card: the card is its layer, not the list", () => {
    const { container } = render(
      <ActionOutcome
        outcome={{ kind: "applied", message: "Becky Lynch can use Operator on their own now" }}
        word="Given"
        trail={<CheckTrail label="On its way" light checks={[done("Recorded."), open("Not sent yet.")]} />}
      />,
    );
    expect(container.querySelector("[data-trail-room]")).toHaveClass("isolate");
    const box = container.querySelector("[data-trail]")!;
    expect(box).toHaveClass("relative");
    expect(box).not.toHaveClass("isolate");
    expect(container.querySelector("ol")).not.toHaveClass("isolate");
    // And a lit result holds no link: a violet way on would sit under 4.5:1 at the light's centre (contrast.test.ts).
    expect(container.querySelector("a")).toBeNull();
  });
});

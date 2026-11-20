// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangeTrail, TRAIL_WATCH_MS, changeTrailFor, trailSteps } from "@/components/people/ChangeTrail";
import { CheckTrail, type TrailCheck } from "@/components/ui/CheckTrail";
import { motionInTests } from "@/test-utils/motion";
import { SRC, css } from "./cssTokens";

// A-25 moment 5, the trail lights (owner, 20 Nov 2026: the recommendation): under a give's result, a
// light rests on the step the change has reached, moves on only when a read settles the next, and goes
// out when the change is home, or when the trail stops looking.

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
const give = (act: "give" | "revoke" = "give", sent = false) => (
  <ChangeTrail userId="u1" projectId="p1" roleKey="operator" act={act} sent={sent} at={at} />
);
const light = () => document.querySelector<HTMLElement>("[data-trail-light]");
const steps = () => screen.getAllByRole("listitem");
const current = () => steps().findIndex((li) => li.getAttribute("aria-current") === "step");

// jsdom lays nothing out: each step is 36px under the last, so where the light rests can be read.
const STEP = 36;
beforeEach(() => {
  at = Date.now();
  person.standing = "";
  person.readAt = "2026-11-20T09:00:00Z";
  person.updatedAt = at + 1;
  vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) {
    return this.tagName === "LI" ? [...this.parentElement!.children].filter((c) => c.tagName === "LI").indexOf(this) * STEP : 0;
  });
});
afterEach(() => vi.restoreAllMocks());

const open = (sentence: string): TrailCheck => ({ tone: "neutral", ring: true, sentence });
const done = (sentence: string): TrailCheck => ({ tone: "healthy", sentence });

describe("the light rests on the step the change has reached", () => {
  it("is on the first open step only: one light, one current step", () => {
    render(<CheckTrail label="On its way" light checks={[done("Recorded."), open("Not sent yet."), open("Not in yet.")]} />);
    expect(document.querySelectorAll("[data-trail-light]")).toHaveLength(1);
    expect(current()).toBe(1);
    expect(steps().filter((li) => li.hasAttribute("aria-current"))).toHaveLength(1);
    expect(light()).toHaveAttribute("data-going");
    // On that step's dot: 11px down its 36px row (the dot's 8px and half its 6px).
    expect(light()!.style.transform).toMatch(new RegExp(`^translate\\(-20px, ${STEP + 11}px\\)$`));
  });

  it("moves only after a read: time alone moves nothing, and a read that settles the next step moves it there", () => {
    const view = render(give());
    expect(current()).toBe(1);
    const resting = light()!.style.transform;
    // Nothing new read: a re-render leaves it where it is.
    view.rerender(give());
    expect(current()).toBe(1);
    expect(light()!.style.transform).toBe(resting);
    // A read that shows Zitadel took it: "Sent" settles, and the light moves to the next step.
    person.standing = "sent";
    view.rerender(give());
    expect(current()).toBe(2);
    expect(light()!.style.transform).toMatch(new RegExp(`, ${2 * STEP + 11}px\\)$`));
  });

  it("goes out when the last step is read back: the change is home", () => {
    const view = render(give("give", true));
    expect(light()).toHaveAttribute("data-going");
    person.standing = "held";
    person.readAt = "2026-11-20T09:05:00Z";
    view.rerender(give("give", true));
    expect(current()).toBe(-1);
    // It stays where it rested, to go out there.
    expect(light()).not.toHaveAttribute("data-going");
    expect(light()).not.toHaveAttribute("data-breathe");
  });

  it("goes out when the trail stops looking, after TRAIL_WATCH_MS, though the step is still open", () => {
    vi.useFakeTimers();
    try {
      at = Date.now();
      person.updatedAt = at + 1;
      render(give("give", true));
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

describe("each dot fills a beat behind the light", () => {
  it("wears the light's lead on a lit trail, and not on any other", () => {
    render(<CheckTrail light checks={[done("Recorded."), open("Not sent yet.")]} />);
    const dots = [...document.querySelectorAll("[data-ring], ol .rounded-pill")];
    expect(dots.length).toBeGreaterThan(0);
    for (const dot of dots) expect(dot).toHaveClass("light-lead");
    expect(/@utility light-lead \{\s*transition-delay: var\(--light-lead\);\s*\}/.test(css)).toBe(true);
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

describe("where it never is", () => {
  it("has no trail at all on a refusal or a failure", () => {
    expect(changeTrailFor("failed", undefined)).toBeUndefined();
    expect(changeTrailFor("pending", "refused:held")).toBeUndefined();
  });

  it("is not on a removal's trail: nothing that takes access away is lit", () => {
    render(give("revoke"));
    expect(light()).toBeNull();
    expect(current()).toBe(-1);
    expect(document.querySelector(".light-lead")).toBeNull();
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
    render(<CheckTrail light checks={[done("Recorded."), open("Not sent yet.")]} />);
    expect(light()).toHaveClass("trail-light-glide");
    expect(light()).toHaveAttribute("data-breathe");
  });

  it("neither travels nor breathes when still (reduced motion): it rests on the step", () => {
    motionInTests(false);
    render(<CheckTrail light checks={[done("Recorded."), open("Not sent yet.")]} />);
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
});

describe("no box cuts it (rule 1): its geometry in a dialog", () => {
  it("is spent inside the dialog's 24px margin, above the footer's ground and below the outcome's", () => {
    // A give's dialog, laid out: the outcome ends at 88, the trail (24px in from the panel's inner
    // edge, 360 wide) starts 12 under it, three steps 36 apart, the last one line (22.5px), and the
    // footer's ground begins 4 + 8 under the trail.
    const box = (top: number, height: number, left = 24, width = 360) =>
      ({ top, bottom: top + height, left, right: left + width, width, height, x: left, y: top, toJSON() {} }) as DOMRect;
    const trailTop = 100;
    const trailHeight = 2 * STEP + 22.5;
    const footerTop = trailTop + trailHeight + 4 + 8;
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
      if (this.matches("[data-testid=outcome]")) return box(40, 48);
      if (this.matches("[data-testid=footer]")) return box(footerTop, 80, 0, 408);
      if (this.matches("[data-trail-room], ol")) return box(trailTop, trailHeight);
      return box(0, 0);
    });
    const dialog = (checks: TrailCheck[]) => (
      <div>
        <div data-testid="outcome" />
        <div data-trail-room className="mx-6 mb-1 mt-3">
          <CheckTrail label="On its way" light checks={checks} />
        </div>
        <div data-testid="footer" />
      </div>
    );
    for (const [name, checks, step] of [
      ["the second step", [done("Recorded."), open("Not sent yet."), open("Not in yet.")], 1],
      ["the last step", [done("Recorded."), done("Sent."), open("Not read back yet.")], 2],
    ] as const) {
      const view = render(dialog([...checks]));
      const el = light()!;
      const [, x, y] = /translate\((-?[\d.]+)px, (-?[\d.]+)px\)/.exec(el.style.transform)!.map(Number);
      const top = trailTop + y;
      const bottom = top + parseFloat(el.style.height);
      // Centred on the open step's dot.
      expect((top + bottom) / 2, name).toBeCloseTo(trailTop + step * STEP + 11);
      // Its left 20px outside the trail: inside the dialog's 24px margin.
      expect(x, name).toBe(-20);
      // Never past the outcome above or the footer below: their grounds would end it at strength.
      expect(top, name).toBeGreaterThanOrEqual(88);
      expect(bottom, name).toBeLessThanOrEqual(footerTop);
      // Never wider than the trail and its 20px.
      expect(parseFloat(el.style.width), name).toBeLessThanOrEqual(360 + 20);
      view.unmount();
    }
  });
});

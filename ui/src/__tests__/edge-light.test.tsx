// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HAND_FIRST, HandHost } from "@/components/shell/HandHost";
import { Button, buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { block, colour, css, over, ratio, type RGBA } from "./cssTokens";

// A-25 moment 2, the edge (owner, 20 Nov 2026: "i really like the edge"): the hairline of the thing
// your hand is on, lit where your hand is nearest, on cards and on outline, ghost and filled buttons,
// by one utility and one listener.

const utility = (name: string) => {
  const start = css.indexOf(`@utility ${name} {`);
  expect(start, name).toBeGreaterThan(-1);
  return css.slice(start, css.indexOf("\n}\n", start) + 2);
};
const rim = utility("rim");

/** A pointer's move, as a browser sends it (jsdom has no PointerEvent). */
function move(target: Element, x: number, y: number, pointerType = "mouse") {
  const event = new MouseEvent("pointermove", { bubbles: true, clientX: x, clientY: y });
  Object.defineProperty(event, "pointerType", { value: pointerType });
  target.dispatchEvent(event);
}

let frames: FrameRequestCallback[] = [];
const flush = () => {
  const due = frames;
  frames = [];
  due.forEach((frame) => frame(0));
};

beforeEach(() => {
  frames = [];
  vi.spyOn(window, "requestAnimationFrame").mockImplementation((frame) => {
    frames.push(frame);
    return frames.length;
  });
  vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => {});
});
afterEach(() => vi.restoreAllMocks());

/** A Home-like page: the shell's scope, a card with rows of buttons, and the one host. */
function page({ scope = true, rows = 12 }: { scope?: boolean; rows?: number } = {}) {
  return render(
    <main data-hand={scope ? "" : undefined}>
      <HandHost />
      <Card data-testid="card">
        {Array.from({ length: rows }, (_, i) => (
          <Button key={i} variant={i % 3 === 0 ? "accent" : i % 3 === 1 ? "outline" : "ghost"}>
            Row {i}
          </Button>
        ))}
      </Card>
      <Card>
        <Button>Another card</Button>
      </Card>
    </main>,
  );
}

describe("the light under your hand: one listener, for cards and buttons", () => {
  it("is one passive pointermove on the window, however many wearers, and none on any element", () => {
    const onWindow = vi.spyOn(window, "addEventListener");
    const onElements = vi.spyOn(Element.prototype, "addEventListener");
    page({ rows: 40 });
    const moves = onWindow.mock.calls.filter(([type]) => type === "pointermove");
    expect(moves).toHaveLength(1);
    expect(moves[0][2]).toMatchObject({ passive: true });
    // React's own delegation sits on its root container; no wearer, and nothing on the page, listens.
    const listeners = onElements.mock.calls
      .map((call, i) => [call[0], onElements.mock.contexts[i]] as const)
      .filter(([type, on]) => type === "pointermove" && on instanceof Element && on.matches("main, main *"));
    expect(listeners).toHaveLength(0);
  });

  it("lays nothing out on pointermove: it measures once a frame, every wearer before any write", () => {
    const view = page();
    const button = view.getAllByRole("button")[4];
    const card = view.getByTestId("card");
    const measured: Element[] = [];
    const written: string[] = [];
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
      measured.push(this);
      return { left: this === card ? 100 : 140, top: this === card ? 50 : 70, right: 0, bottom: 0, width: 0, height: 0, x: 0, y: 0, toJSON() {} } as DOMRect;
    });
    const set = CSSStyleDeclaration.prototype.setProperty;
    vi.spyOn(CSSStyleDeclaration.prototype, "setProperty").mockImplementation(function (this: CSSStyleDeclaration, name, value) {
      written.push(`${name}:${measured.length}`);
      return set.call(this, name, value);
    });
    // Three moves in one frame: nothing is measured or written while they are handled.
    move(button, 150, 80);
    move(button, 160, 82);
    move(button, 170, 84);
    expect(measured).toEqual([]);
    expect(written).toEqual([]);
    expect(frames).toHaveLength(1);
    flush();
    // The button, then its card: each measured once, both before the first write.
    expect(measured).toEqual([button, card]);
    expect(written.every((w) => w.endsWith(":2"))).toBe(true);
    expect(button.style.getPropertyValue("--px")).toBe("30px");
    expect(button.style.getPropertyValue("--py")).toBe("14px");
    expect(card.style.getPropertyValue("--px")).toBe("70px");
    expect(card.style.getPropertyValue("--py")).toBe("34px");
  });

  it("follows no touch, and nothing outside the page's scope or inside a Notice", () => {
    const view = page({ scope: false });
    move(view.getAllByRole("button")[0], 10, 10);
    flush();
    expect(view.getAllByRole("button")[0].style.getPropertyValue("--px")).toBe("");
    view.unmount();
    const scoped = page();
    move(scoped.getAllByRole("button")[0], 10, 10, "touch");
    expect(frames).toEqual([]);
    scoped.unmount();
    const notice = render(
      <main data-hand="">
        <HandHost />
        <div data-no-hand="">
          <Button>Try again</Button>
        </div>
      </main>,
    );
    move(notice.getByRole("button"), 10, 10);
    flush();
    expect(notice.getByRole("button").style.getPropertyValue("--px")).toBe("");
  });

  it("ships first on Home and People (A-25's build order)", () => {
    for (const path of ["/", "/users", "/users/u1"]) expect(HAND_FIRST.test(path), path).toBe(true);
    for (const path of ["/requests", "/roles", "/userspace", "/me"]) expect(HAND_FIRST.test(path), path).toBe(false);
  });
});

describe("who wears it", () => {
  it("is outline, ghost and filled, never a destructive or a disabled button", () => {
    expect(buttonClasses({ variant: "accent", size: "md" })).toMatch(/\brim rim-filled\b/);
    expect(buttonClasses({ variant: "outline", size: "md" })).toMatch(/\brim rim-outline\b/);
    expect(buttonClasses({ variant: "ghost", size: "md" })).toMatch(/\brim\b/);
    for (const variant of ["danger", "dangerConfirm"] as const) {
      expect(buttonClasses({ variant, size: "md" }), variant).not.toMatch(/\brim/);
    }
    for (const variant of ["accent", "outline", "ghost", "danger", "dangerConfirm"] as const) {
      expect(buttonClasses({ variant, size: "md", disabled: true }), `${variant} disabled`).not.toMatch(/\brim/);
    }
    // A pending button is a disabled one.
    const pending = render(<Button isPending>Saving</Button>);
    expect(pending.getByRole("button").className).not.toMatch(/\brim/);
  });

  it("is a card's own hairline, at inset 0 inside its border (a card clips at its padding edge), never on a stale card", () => {
    const card = render(<Card data-testid="c" />).getByTestId("c");
    expect(card).toHaveClass("card", "overflow-hidden", "rim", "rim-card");
    // The ring's place: the utility's own default, 0, and the card's kind never moves it.
    expect(rim).toMatch(/inset: var\(--rim-inset, 0px\);/);
    expect(utility("rim-card")).not.toMatch(/--rim-inset/);
    expect(utility("rim-outline")).toMatch(/--rim-inset: -1px;/);
    expect(utility("rim-filled")).toMatch(/--rim-inset: 1px;/);
    const stale = render(<Card data-testid="s" stale />).getByTestId("s");
    expect(stale).not.toHaveClass("rim");
  });
});

describe("what it draws", () => {
  it("is a one-pixel ring on the edge, nothing in the fill and so nothing behind a word, in on tint and out on settle", () => {
    expect(rim).toMatch(/padding: 1px;/);
    expect(rim).toMatch(/mask: linear-gradient\(#000 0 0\) content-box exclude, linear-gradient\(#000 0 0\);/);
    expect(rim).toMatch(/border-radius: inherit;/);
    expect(rim).toMatch(/pointer-events: none;/);
    expect(rim).toMatch(/background: radial-gradient\(circle var\(--rim-r, 84px\) at var\(--px, 50%\) var\(--py, 50%\), var\(--pool-stops\)\);/);
    expect(rim).toMatch(/opacity: 0;\s*transition: opacity var\(--duration-settle\) var\(--ease-tint\);/);
    expect(rim).toMatch(/@media \(hover: hover\) \{\s*&:hover::after \{\s*opacity: 1;\s*transition-duration: var\(--duration-tint\);/);
    expect(rim).toMatch(/@media \(forced-colors: active\) \{\s*&::after \{\s*display: none;/);
    // Only inside the page's scope, never in a Notice or a closed part, never over a first-day card's own edge.
    // Every state of it under that one scope: the ring, the hover, the keyboard's.
    const scope = ":where([data-hand]) &:not(.lit-card, :where([data-no-hand] *)) {";
    expect(rim).toContain(scope);
    const scoped = rim.slice(rim.indexOf(scope));
    for (const state of ["&::after {", "&:hover::after {", "&:focus-visible::after {", "&.rim-card:has(:focus-visible)::after {"]) {
      expect(scoped, state).toContain(state);
    }
    expect(rim.slice(0, rim.indexOf(scope))).not.toContain("::after");
    // Its colour: the accent, and on a filled button its own ink, by token.
    expect(utility("rim-filled")).toMatch(/--pool-c: var\(--rim-filled\);/);
    expect(css).toMatch(/--rim-filled: var\(--accent-ink\);/);
  });

  it("holds A-25's measured ratios: never fainter than the strong hairline on a card, never louder than a whole lit edge", () => {
    // A-25 moment 2, measured on the prototype (dark room, then paper): card 1.7 to 1.8 and 1.8 to
    // 1.9 beside the hand, the strong hairline 1.5 and 1.4; outline 2.2 and 2.1 under the hand; ghost
    // 1.6; filled 1.6 and 1.75 against its face; the whole-edge option B 2.6 and up. Here at each
    // pool's strongest point, so at or a little above what was measured.
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const strength = (name: string) => Number(new RegExp(`--rim-k: (\\d+)%;`).exec(utility(name))?.[1] ?? /--rim-k, (\d+)%/.exec(rim)![1]) / 100;
    const lit = (c: RGBA, k: number, under: RGBA) => over([c[0], c[1], c[2], (c[3] ?? 1) * k], under);
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const canvas = colour(vars, "--canvas");
      const surface = colour(vars, "--surface-1");
      const hairline = ratio(over(colour(vars, "--line-strong"), surface), surface);
      const edge = over(colour(vars, "--line-strong"), canvas);
      const face = colour(vars, "--accent-dense");
      const card = ratio(lit(accent, strength("rim-card"), surface), surface);
      const outline = ratio(lit(accent, strength("rim-outline"), edge), canvas);
      const ghost = ratio(lit(accent, strength("rim"), canvas), canvas);
      const filled = ratio(lit(colour(vars, "--accent-ink"), strength("rim-filled"), face), face);
      expect(card, `${theme} card`).toBeGreaterThanOrEqual(hairline);
      expect(card, `${theme} card`).toBeLessThan(2.2);
      expect(outline, `${theme} outline`).toBeGreaterThan(ratio(edge, canvas));
      expect(outline, `${theme} outline`).toBeLessThan(2.6);
      expect(ghost, `${theme} ghost`).toBeGreaterThanOrEqual(1.5);
      expect(ghost, `${theme} ghost`).toBeLessThan(2.6);
      expect(filled, `${theme} filled`).toBeGreaterThanOrEqual(1.5);
      expect(filled, `${theme} filled`).toBeLessThan(2.2);
    }
  });
});

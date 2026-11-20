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
/** The block a selector opens, braces balanced. */
function braced(source: string, opener: string): string {
  const start = source.indexOf(opener);
  expect(start, opener).toBeGreaterThan(-1);
  let depth = 0;
  for (let i = source.indexOf("{", start); i < source.length; i++) {
    depth += source[i] === "{" ? 1 : source[i] === "}" ? -1 : 0;
    if (depth === 0) return source.slice(start, i + 1);
  }
  return source.slice(start);
}
const SCOPE = ":where([data-hand]) &:not(.lit-card, :where([data-no-hand] *)) {";
const scoped = braced(rim, SCOPE);
const unscoped = rim.replace(scoped, "");

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
/** What the browser would hit at the pointer: jsdom does no layout, so the test says. */
let hit: Element | null = null;

beforeEach(() => {
  frames = [];
  hit = null;
  vi.spyOn(window, "requestAnimationFrame").mockImplementation((frame) => {
    frames.push(frame);
    return frames.length;
  });
  vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => {});
  document.elementFromPoint = () => hit;
});
afterEach(() => vi.restoreAllMocks());

/** A Home-like page: the shell's scope, two cards with buttons, and the one host. */
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

/** Every box a test hands out: the card at (100, 50), anything else at (140, 70). */
function boxes(card: Element) {
  const measured: Element[] = [];
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    measured.push(this);
    const [left, top] = this === card ? [100, 50] : [140, 70];
    return { left, top, right: 0, bottom: 0, width: 0, height: 0, x: left, y: top, toJSON() {} } as DOMRect;
  });
  return measured;
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
    const measured = boxes(card);
    const written: string[] = [];
    const set = CSSStyleDeclaration.prototype.setProperty;
    vi.spyOn(CSSStyleDeclaration.prototype, "setProperty").mockImplementation(function (this: CSSStyleDeclaration, name, value) {
      written.push(`${name}:${measured.length}`);
      return set.call(this, name, value);
    });
    hit = button;
    // Three moves in one frame: nothing is measured or written while they are handled.
    move(button, 150, 80);
    move(button, 160, 82);
    move(button, 170, 84);
    expect(measured).toEqual([]);
    expect(written).toEqual([]);
    expect(frames).toHaveLength(1);
    flush();
    // The button, then its card: each measured once, both before the first write; the last place.
    expect(measured).toEqual([button, card]);
    expect(written.every((w) => w.endsWith(":2"))).toBe(true);
    expect(button.style.getPropertyValue("--px")).toBe("30px");
    expect(button.style.getPropertyValue("--py")).toBe("14px");
    expect(card.style.getPropertyValue("--px")).toBe("70px");
    expect(card.style.getPropertyValue("--py")).toBe("34px");
  });

  it("redraws when the page scrolls or resizes under a still pointer, at what is now under it", () => {
    const view = page();
    const [first, second] = view.getAllByRole("button");
    boxes(view.getByTestId("card"));
    hit = first;
    move(first, 150, 80);
    flush();
    expect(first.style.getPropertyValue("--px")).toBe("10px");
    // The wheel moves the page; the pointer stays. Whatever is under it now is lit there.
    hit = second;
    view.getByTestId("card").dispatchEvent(new Event("scroll"));
    expect(frames).toHaveLength(1);
    flush();
    expect(second.style.getPropertyValue("--px")).toBe("10px");
    window.dispatchEvent(new Event("resize"));
    expect(frames).toHaveLength(1);
  });

  it("lets go of everything when it unmounts", () => {
    const added = vi.spyOn(window, "addEventListener");
    const removed = vi.spyOn(window, "removeEventListener");
    page().unmount();
    const ours = added.mock.calls.filter(([type]) => ["pointermove", "scroll", "resize", "focusin"].includes(type as string));
    expect(ours.map(([type]) => type).sort()).toEqual(["focusin", "pointermove", "resize", "scroll"]);
    for (const [type, listener] of ours) {
      expect(removed.mock.calls.some(([t, l]) => t === type && l === listener), String(type)).toBe(true);
    }
  });

  it("follows no touch, and nothing outside the page's scope", () => {
    const view = page({ scope: false });
    hit = view.getAllByRole("button")[0];
    move(hit, 10, 10);
    flush();
    expect(view.getAllByRole("button")[0].style.getPropertyValue("--px")).toBe("");
    view.unmount();
    const scoped = page();
    move(scoped.getAllByRole("button")[0], 10, 10, "touch");
    expect(frames).toEqual([]);
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
    expect(rim).toMatch(/^\s*--rim-inset: 0px;/m);
    expect(braced(rim, "&.rim-card {")).not.toMatch(/--rim-inset/);
    expect(braced(rim, "&.rim-outline {")).toMatch(/--rim-inset: -1px;/);
    expect(braced(rim, "&.rim-filled {")).toMatch(/--rim-inset: 1px;/);
    const stale = render(<Card data-testid="s" stale />).getByTestId("s");
    expect(stale).not.toHaveClass("rim");
  });

  it("is each wearer's own: a button inside a card never takes the card's strength, reach or place", () => {
    // Declared on every wearer, so nothing inherits from the card around it; the kinds are compound
    // (`.rim.rim-card`), decided by specificity, not by the order the CSS is written in.
    for (const own of ["--rim-k: 46%;", "--rim-r: 84px;", "--rim-inset: 0px;"]) expect(unscoped, own).toContain(own);
    expect(css).not.toMatch(/@utility rim-(card|outline|filled)\b/);
    for (const kind of ["rim-card", "rim-outline", "rim-filled"]) expect(unscoped, kind).toContain(`&.${kind} {`);
    // The pointer's place is not inherited: a frame's write restyles the wearer alone.
    expect(css).toMatch(/@property --px \{ syntax: "<length-percentage>"; inherits: false; initial-value: 50%; \}/);
    expect(css).toMatch(/@property --py \{ syntax: "<length-percentage>"; inherits: false; initial-value: 50%; \}/);
    expect(braced(scoped, "&::after {")).toMatch(/--px: inherit;\s*--py: inherit;/);
  });

  it("leaves a first-day card's light as it is: 22% of the accent, by the room's weight, on every route", () => {
    // Nothing outside the drawn scope sets a pool's strength or colour, and the scope is never a lit card.
    expect(unscoped).not.toMatch(/--pool-[a-z]+:/);
    expect(SCOPE).toContain(":not(.lit-card");
    // So a lit card's strength is its own: --lit-reach times --hour-weight, in both rooms.
    const lit = utility("lit-card");
    expect(lit).toMatch(/--pool-k: calc\(var\(--lit-reach\) \* var\(--hour-weight\)\);/);
    const reach = Number(/--lit-reach: (\d+)%;/.exec(lit)?.[1]);
    expect(reach).toBe(22);
    const ramp = block(":root {\n    --violet-600");
    for (const theme of [block(':root,\n  [data-theme="dark"] {'), block('[data-theme="light"] {')]) {
      const weight = Number({ ...ramp, ...theme }["--hour-weight"]);
      expect(reach * weight).toBeLessThan(25);
    }
  });
});

describe("what it draws", () => {
  it("is drawn only where the page says, never inside a Notice or a closed part, never under forced colours, only for a hand that hovers", () => {
    for (const state of ["&::after {", "&:hover::after {", "&:focus-visible::after {", "&.rim-card:has(:focus-visible)::after {"]) {
      expect(scoped, state).toContain(state);
    }
    expect(unscoped).not.toMatch(/::after \{\s*content/);
    expect(SCOPE).toContain(":where([data-no-hand] *)");
    expect(scoped).toMatch(/@media \(hover: hover\) \{\s*&:hover::after/);
    expect(rim).toMatch(/@media \(forced-colors: active\) \{\s*&::after \{\s*display: none;/);
  });

  it("holds A-25's measured ratios: never fainter than the strong hairline on a card, never louder than a whole lit edge", () => {
    // A-25 moment 2, measured on the prototype (dark room, then paper): card 1.7 to 1.8 and 1.8 to
    // 1.9 beside the hand, the strong hairline 1.5 and 1.4; outline 2.2 and 2.1 under the hand; ghost
    // 1.6; filled 1.6 and 1.75 against its face; the whole-edge option B 2.6 and up. Here at each
    // pool's strongest point, so at or a little above what was measured.
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const strength = (kind?: string) =>
      Number(/--rim-k: (\d+)%;/.exec(kind ? braced(rim, `&.${kind} {`) : unscoped)?.[1] ?? /--rim-k: (\d+)%;/.exec(unscoped)![1]) / 100;
    // The filled button's colour, by its token.
    expect(braced(scoped, "&.rim-filled {")).toMatch(/--pool-c: var\(--rim-filled\);/);
    const filledInk = /--rim-filled: var\((--[\w-]+)\);/.exec(css)![1];
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
      const ghost = ratio(lit(accent, strength(), canvas), canvas);
      const filled = ratio(lit(colour(vars, filledInk), strength("rim-filled"), face), face);
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

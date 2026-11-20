// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HandHost, WEARERS } from "@/components/shell/HandHost";
import { Button, buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { StatCard } from "@/components/ui/StatCard";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { SRC, block, colour, css, over, ratio, type RGBA } from "./cssTokens";

// A-25 moment 2, the edge (owner, 20 Nov 2026: "i really like the edge"): the hairline of the thing
// your hand is on, lit where your hand is nearest, on cards and on outline, ghost and filled buttons,
// by one edge in globals.css, worn by every card frame, and one listener.

const utility = (name: string) => {
  const start = css.indexOf(`@utility ${name} {`);
  expect(start, name).toBeGreaterThan(-1);
  return css.slice(start, css.indexOf("\n}\n", start) + 2);
};
/** The block a selector opens, braces balanced; a selector that is not there fails the file, never a test vacuously. */
function braced(source: string, opener: string, from = source.indexOf(opener)): string {
  if (from < 0) throw new Error(`not in the CSS: ${opener}`);
  const start = from;
  let depth = 0;
  for (let i = source.indexOf("{", start); i < source.length; i++) {
    depth += source[i] === "{" ? 1 : source[i] === "}" ? -1 : 0;
    if (depth === 0) return source.slice(start, i + 1);
  }
  return source.slice(start);
}
const OWN = ":where(.rim, .card, .panel) {";
/** The edge, one block in globals.css (the layer that opens with its wearers' own values): its kinds, and what it draws where. */
const edge = braced(css, "@layer components {", css.lastIndexOf("@layer components {", css.indexOf(OWN)));
const CARD_KIND = ":is(.card, .panel, .rim-card) {";
const NOT = '.lit-card, [data-stale], [data-no-hand], [data-no-hand] *, :has(> [aria-busy="true"])';
const SCOPE = `:where([data-hand]) :is(.rim, .card, .panel):not(${NOT}) {`;
/** What the edge draws outside its scope: every wearer's own values and kinds. */
const unscopedBlock = () => edge.replace(scopedBlock(), "");
/** The rule's own selector, read from the CSS, and the block it opens. */
const drawnSelector = () => /(:where\(\[data-hand\]\) :is\(\.rim, \.card, \.panel\):not\(.*\)) \{/.exec(edge)?.[1] ?? "";
const scopedBlock = () => braced(edge, SCOPE);
/** The frame utilities: any that draws a card's radius, a surface and a one-pixel hairline. */
const FRAMES = [...css.matchAll(/@utility ([\w-]+) \{([^}]*)\}/g)]
  .filter(([, , body]) => /border-radius: var\(--radius-(card|panel)\)/.test(body) && /background-color: var\(--surface-\d\)/.test(body) && /border: 1px solid/.test(body))
  .map(([, name]) => name);

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
    const inScope = page();
    move(inScope.getAllByRole("button")[0], 10, 10, "touch");
    expect(frames).toEqual([]);
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
    expect(card).toHaveClass("card", "overflow-hidden");
    expect(card.matches(WEARERS)).toBe(true);
    expect(braced(edge, OWN)).toMatch(/--rim-inset: 0px;/);
    expect(braced(edge, CARD_KIND)).not.toMatch(/--rim-inset/);
    expect(braced(edge, ".rim.rim-outline {")).toMatch(/--rim-inset: -1px;/);
    expect(braced(edge, ".rim.rim-filled {")).toMatch(/--rim-inset: 1px;/);
    // A stale card recedes: the scope refuses it.
    const stale = render(<Card data-testid="s" stale />).getByTestId("s");
    expect(stale).toHaveAttribute("data-stale");
    expect(SCOPE).toContain("[data-stale]");
  });

  it("is a stat tile's too, a link or not, with no flat ring of its own (owner on staging, 20 Nov: \"these dont show the effect\")", () => {
    const linked = render(<StatCard label="Connected systems" value="3" detail="All answering." href="/system" />);
    const tile = linked.getByRole("link", { name: "Connected systems" });
    expect(tile.matches(WEARERS)).toBe(true);
    expect(tile.className).not.toMatch(/hover:shadow/);
    const still = render(<StatCard label="Members" value="120" detail="In Zitadel." />);
    expect(still.container.querySelector("[data-stat-tile]")!.matches(WEARERS)).toBe(true);
  });

  it("is carried by every frame utility, so no frame can be drawn without it", () => {
    // Every utility that draws a card's radius, a surface and a hairline is a wearer, in CSS and in the listener.
    expect(FRAMES.sort()).toEqual(["card", "panel"]);
    // Read from the CSS itself: who has the edge's own values, who lights like a card, who is drawn.
    const listed = (re: RegExp) => (re.exec(edge)?.[1] ?? "").split(",").map((part) => part.trim());
    const own = listed(/:where\(([^)]*)\) \{\s*position: relative;/);
    const kind = listed(/:is\(([^)]*)\) \{\s*--rim-k: 50%;/);
    const drawn = listed(/:where\(\[data-hand\]\) :is\(([^)]*)\):not\(/);
    for (const frame of FRAMES) {
      expect(own, frame).toContain(`.${frame}`);
      expect(kind, frame).toContain(`.${frame}`);
      expect(drawn, frame).toContain(`.${frame}`);
      expect(WEARERS.split(", "), frame).toContain(`.${frame}`);
    }
    expect(WEARERS.split(", ").sort()).toEqual(drawn.sort());
  });

  it("lets a light's own colour win: every --pool-* default sits in a lower cascade layer than every override", () => {
    // Where each declaration of a pool's colour or strength lives, by the cascade's own order: base,
    // components, utilities (Tailwind's @utility), then unlayered, which beats every layer whatever its
    // specificity. A default unlayered was why a filled button's edge lit in the accent on its accent fill.
    const RANK = { base: 1, components: 2, utilities: 3, unlayered: 4 } as const;
    const layerAt = (index: number): keyof typeof RANK => {
      let depth = 0;
      let opener = "";
      for (let i = 0; i < index; i++) {
        if (css[i] === "{") {
          if (depth === 0) opener = css.slice(css.lastIndexOf("\n", i) + 1, i);
          depth++;
        } else if (css[i] === "}") depth--;
      }
      if (depth === 0) return "unlayered";
      if (/@layer base/.test(opener)) return "base";
      if (/@layer components/.test(opener)) return "components";
      if (/@utility /.test(opener)) return "utilities";
      return "unlayered";
    };
    for (const prop of ["--pool-c", "--pool-k", "--pool-stops"]) {
      const found = [...css.matchAll(new RegExp(`^\\s*${prop}:`, "gm"))].map((m) => {
        const before = css.slice(0, m.index);
        const selector = before.slice(before.lastIndexOf("{", m.index) - 200, before.lastIndexOf("{"));
        return { rank: RANK[layerAt(m.index!)], isDefault: /:where\(\.hour-light/.test(selector) };
      });
      const defaults = found.filter((f) => f.isDefault);
      const overrides = found.filter((f) => !f.isDefault);
      for (const d of defaults) for (const o of overrides) expect(d.rank, prop).toBeLessThan(o.rank);
    }
    // And the pool's default is not unlayered at all.
    expect(layerAt(css.indexOf(":where(.hour-light, .lit-card,"))).toBe("base");
  });

  it("answers no hand with a flat ring", () => {
    const files: string[] = [];
    const walk = (dir: string) => {
      for (const name of readdirSync(dir)) {
        const path = join(dir, name);
        if (statSync(path).isDirectory()) {
          if (name !== "__tests__" && name !== "node_modules") walk(path);
        } else if (/\.tsx?$/.test(name)) files.push(path);
      }
    };
    walk(SRC);
    const flatRings: string[] = [];
    for (const path of files) {
      readFileSync(path, "utf8").split("\n").forEach((line, i) => {
        // The old language, a flat accent ring all round under the hand: banned, the edge replaces it.
        if (/hover:shadow-\[0_0_0_1px|hover:border-accent/.test(line)) flatRings.push(`${path.slice(SRC.length + 1)}:${i + 1}`);
      });
    }
    expect(flatRings).toEqual([]);
  });

  it("is drawn on a frame, and never on one that refuses it, sits inside one, is a first-day card, is stale, or is still a placeholder", () => {
    // The rule's own `:not()`: the frame carrying `data-no-hand` itself, not only what is inside one.
    const drawn = drawnSelector();
    expect(drawn).toMatch(/:not\([^)]*\[data-no-hand\],/);
    const view = render(
      <main data-hand="">
        <Card data-testid="lit" />
        <Card data-testid="refuses" data-no-hand="" />
        <div data-no-hand="">
          <Card data-testid="inside" />
        </div>
        <Card data-testid="first-day" className="lit-card" />
        <Card data-testid="stale" stale />
        <Card data-testid="placeholder">
          <div aria-busy="true" />
        </Card>
        <div data-testid="panel" className="panel" />
        <Button>Act</Button>
      </main>,
    );
    const is = (id: string) => view.getByTestId(id).matches(drawn);
    expect(is("lit")).toBe(true);
    expect(is("panel")).toBe(true);
    expect(view.getByRole("button").matches(drawn)).toBe(true);
    for (const id of ["refuses", "inside", "first-day", "stale", "placeholder"]) expect(is(id), id).toBe(false);
  });

  it("is each wearer's own: a button inside a card never takes the card's strength, reach or place", () => {
    // Declared on every wearer, so nothing inherits from the card around it; the kinds are compound
    // (`.rim.rim-card`), decided by specificity, not by the order the CSS is written in.
    for (const own of ["--rim-k: 46%;", "--rim-r: 84px;", "--rim-inset: 0px;"]) expect(braced(edge, OWN), own).toContain(own);
    expect(css).not.toMatch(/@utility rim(-card|-outline|-filled)?\b/);
    for (const kind of [CARD_KIND, ".rim.rim-outline {", ".rim.rim-filled {"]) expect(unscopedBlock(), kind).toContain(kind);
    // The pointer's place is not inherited: a frame's write restyles the wearer alone.
    expect(css).toMatch(/@property --px \{ syntax: "<length-percentage>"; inherits: false; initial-value: 50%; \}/);
    expect(css).toMatch(/@property --py \{ syntax: "<length-percentage>"; inherits: false; initial-value: 50%; \}/);
    expect(braced(scopedBlock(), "&::after {")).toMatch(/--px: inherit;\s*--py: inherit;/);
  });

  it("leaves a first-day card's light as it is: 22% of the accent, by the room's weight, on every route", () => {
    // Nothing outside the drawn scope sets a pool's strength or colour, and the scope is never a lit card.
    expect(unscopedBlock()).not.toMatch(/--pool-[a-z]+:/);
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
    for (const state of ["&::after {", "&:hover::after {", "&:focus-visible::after {", "&:is(.card, .panel, .rim-card):has(:focus-visible)::after {"]) {
      expect(scopedBlock(), state).toContain(state);
    }
    expect(unscopedBlock()).not.toMatch(/::after \{\s*content/);
    expect(SCOPE).toContain("[data-no-hand] *");
    expect(scopedBlock()).toMatch(/@media \(hover: hover\) \{\s*&:hover::after/);
    expect(edge).toMatch(/@media \(forced-colors: active\) \{\s*:is\(\.rim, \.card, \.panel\)::after \{\s*display: none;/);
  });

  it("holds A-25's measured ratios: never fainter than the strong hairline on a card, never louder than a whole lit edge", () => {
    // A-25 moment 2, measured on the prototype (dark room, then paper): card 1.7 to 1.8 and 1.8 to
    // 1.9 beside the hand, the strong hairline 1.5 and 1.4; outline 2.2 and 2.1 under the hand; ghost
    // 1.6; filled 1.6 and 1.75 against its face; the whole-edge option B 2.6 and up. Here at each
    // pool's strongest point, so at or a little above what was measured.
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const strength = (kind?: string) =>
      Number(/--rim-k: (\d+)%;/.exec(braced(edge, kind ?? OWN))?.[1] ?? /--rim-k: (\d+)%;/.exec(braced(edge, OWN))![1]) / 100;
    // The filled button's colour, by its token.
    expect(braced(scopedBlock(), "&.rim-filled {")).toMatch(/--pool-c: var\(--rim-filled\);/);
    const filledInk = /--rim-filled: var\((--[\w-]+)\);/.exec(css)![1];
    const lit = (c: RGBA, k: number, under: RGBA) => over([c[0], c[1], c[2], (c[3] ?? 1) * k], under);
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const canvas = colour(vars, "--canvas");
      const surface = colour(vars, "--surface-1");
      const hairline = ratio(over(colour(vars, "--line-strong"), surface), surface);
      const edgeColour = over(colour(vars, "--line-strong"), canvas);
      const face = colour(vars, "--accent-dense");
      const card = ratio(lit(accent, strength(CARD_KIND), surface), surface);
      const outline = ratio(lit(accent, strength(".rim.rim-outline {"), edgeColour), canvas);
      const ghost = ratio(lit(accent, strength(), canvas), canvas);
      const filled = ratio(lit(colour(vars, filledInk), strength(".rim.rim-filled {"), face), face);
      expect(card, `${theme} card`).toBeGreaterThanOrEqual(hairline);
      expect(card, `${theme} card`).toBeLessThan(2.2);
      expect(outline, `${theme} outline`).toBeGreaterThan(ratio(edgeColour, canvas));
      expect(outline, `${theme} outline`).toBeLessThan(2.6);
      expect(ghost, `${theme} ghost`).toBeGreaterThanOrEqual(1.5);
      expect(ghost, `${theme} ghost`).toBeLessThan(2.6);
      expect(filled, `${theme} filled`).toBeGreaterThanOrEqual(1.5);
      expect(filled, `${theme} filled`).toBeLessThan(2.2);
    }
  });
});

// @vitest-environment jsdom
import { act, render, renderHook, screen } from "@testing-library/react";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as motion from "@/lib/motion";
import { installMatchMedia, setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";
import { blockFor, linesFor, withBlock } from "../../../../.design-sync/moves.mjs";

/**
 * A-19 P6: one motion module. The behaviour is the one the components already had
 * (their own tests hold that); this holds the module's shape, its binding to the
 * tokens, and the one rule every verb shares: a still form.
 */

const UI_ROOT = resolve(__dirname, "../../../..");
const GLOBALS = readFileSync(resolve(UI_ROOT, "src/app/globals.css"), "utf8");
const token = (name: string) => Number(new RegExp(`${name}:\\s*(\\d+)ms`).exec(GLOBALS)?.[1]);

describe("one module", () => {
  it("exports every verb from one place", () => {
    for (const name of [
      "stillMotion", "useExitPhase", "glideTo", "GlideIndicator", "useGlideIndicator",
      "ArriveList", "armArrive", "returnedToThisPage", "markViewSwitched",
      "Unfold", "Tinted", "UnfoldOnArrival",
      "LeavingRow", "useLeavingRows", "Crossfade", "useFlashOnChange",
    ]) {
      expect(motion, `@/lib/motion must export ${name}`).toHaveProperty(name);
    }
  });

  it("left no copy of a motion part in components/ui", () => {
    for (const file of ["ArriveList", "Unfold", "LeavingRows", "Crossfade", "GlideIndicator"]) {
      expect(existsSync(resolve(UI_ROOT, `src/components/ui/${file}.tsx`)), `${file}.tsx moved to lib/motion`).toBe(false);
    }
    expect(existsSync(resolve(UI_ROOT, "src/lib/useFlashOnChange.ts"))).toBe(false);
  });
});

describe("the tokens, all of them", () => {
  it("binds every --duration-* in globals.css to a constant a timer uses", () => {
    const bound: Record<string, number> = {
      tint: motion.TINT_MS,
      press: motion.PRESS_MS,
      settle: motion.SETTLE_MS,
      fold: motion.FOLD_MS,
      arrive: motion.ARRIVE_MS,
      flash: motion.FLASH_MS,
      breathe: motion.BREATHE_MS,
      glide: motion.GLIDE_MS,
    };
    const declared = Array.from(GLOBALS.matchAll(/--duration-([a-z]+):\s*\d+ms/g)).map((m) => m[1]);
    expect(declared.sort()).toEqual(Object.keys(bound).sort());
    for (const [name, ms] of Object.entries(bound)) {
      expect(ms, `--duration-${name}`).toBe(token(`--duration-${name}`));
    }
  });

  it("declares every easing the verbs name, and the cap and the stagger", () => {
    for (const ease of ["tint", "press", "settle", "arrive", "breathe", "flash"]) {
      expect(GLOBALS, `--ease-${ease}`).toMatch(new RegExp(`--ease-${ease}:\\s*cubic-bezier\\(`));
    }
    expect(motion.ARRIVE_STEP_MS).toBe(Number(/--arrive-step:\s*(\d+)ms/.exec(GLOBALS)?.[1]));
    expect(motion.ARRIVE_CAP).toBe(Number(/--arrive-cap:\s*(\d+)/.exec(GLOBALS)?.[1]));
    expect(GLOBALS).toContain("--travel-cap: 10px");
  });
});

describe("reduced motion is first-class", () => {
  afterEach(() => {
    vi.useRealTimers();
    installMatchMedia();
  });

  it("is still in a test run until a test opts in", () => {
    expect(motion.stillMotion()).toBe(true);
    motionInTests();
    expect(motion.stillMotion()).toBe(false);
  });

  it("is still when the reader asked for less, even where a test opts in", () => {
    motionInTests();
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    expect(motion.stillMotion()).toBe(true);
  });

  it("an exit phase leaves at once when still, and after its time when not", () => {
    vi.useFakeTimers();
    const phase = (open: boolean) => ({ open });
    const still = renderHook(({ open }) => motion.useExitPhase(open, 260), { initialProps: phase(true) });
    still.rerender(phase(false));
    expect(still.result.current).toEqual({ rendered: false, leaving: false });

    motionInTests();
    const moving = renderHook(({ open }) => motion.useExitPhase(open, 260), { initialProps: phase(true) });
    moving.rerender(phase(false));
    expect(moving.result.current).toEqual({ rendered: true, leaving: true });
    act(() => void vi.advanceTimersByTime(259));
    expect(moving.result.current.rendered).toBe(true);
    act(() => void vi.advanceTimersByTime(1));
    expect(moving.result.current).toEqual({ rendered: false, leaving: false });
  });

  it("a crossfade swaps the words at once when still, and fades the old ones out for a tint when not", () => {
    vi.useFakeTimers();
    const view = render(<motion.Crossfade id="a">before</motion.Crossfade>);
    view.rerender(<motion.Crossfade id="b">after</motion.Crossfade>);
    expect(screen.queryByText("before")).toBeNull();
    expect(screen.getByText("after")).toBeInTheDocument();
    view.unmount();

    motionInTests();
    const moving = render(<motion.Crossfade id="a">before</motion.Crossfade>);
    moving.rerender(<motion.Crossfade id="b">after</motion.Crossfade>);
    expect(screen.getByText("before").closest(".tint-out")).not.toBeNull();
    expect(screen.getByText("after").closest(".tint-in")).not.toBeNull();
    act(() => void vi.advanceTimersByTime(motion.TINT_MS));
    expect(screen.queryByText("before")).toBeNull();
  });

  describe("glide", () => {
    beforeEach(() => {
      document.body.innerHTML = '<div id="app-scroll"></div><h2 id="t">Heading</h2>';
      const scroller = document.getElementById("app-scroll")!;
      scroller.scrollTo = vi.fn() as never;
    });

    it("flashes a region's marked heading row, not the region", async () => {
      document.body.innerHTML =
        '<div id="app-scroll"></div><section id="r"><div data-landing id="row"><h2>Heading</h2></div><p>body</p></section>';
      document.getElementById("app-scroll")!.scrollTo = vi.fn() as never;
      await motion.glideTo(document.getElementById("r")!);
      expect(document.getElementById("row")!.classList.contains("flash")).toBe(true);
      expect(document.getElementById("r")!.classList.contains("flash")).toBe(false);
    });

    it("jumps and still flashes the heading, which is the part the reader needs", async () => {
      const target = document.getElementById("t")!;
      await expect(motion.glideTo(target)).resolves.toBe(true);
      expect((document.getElementById("app-scroll")!.scrollTo as ReturnType<typeof vi.fn>)).toHaveBeenCalledTimes(1);
      expect(target.classList.contains("flash")).toBe(true);
    });

    // The browser's own formula for an anchor jump or a focus: the scroller's scroll-padding-top plus
    // the target's scroll-margin-top, so a glide, a link and Tab land alike.
    it("lands where a native jump would: scroll-padding-top plus scroll-margin-top", async () => {
      const scroller = document.getElementById("app-scroll")!;
      const heading = document.getElementById("t")!;
      const top = () => (scroller.scrollTo as ReturnType<typeof vi.fn>).mock.lastCall![0].top as number;
      await motion.glideTo(heading);
      const bare = top();
      scroller.style.setProperty("scroll-padding-top", "64px");
      await motion.glideTo(heading);
      expect(bare - top()).toBe(64);
      heading.style.setProperty("scroll-margin-top", "16px");
      await motion.glideTo(heading);
      expect(bare - top()).toBe(80);
    });

    it("a later jump takes over a glide in motion, and only the last one lands", async () => {
      motionInTests();
      const target = document.getElementById("t")!;
      const frames: FrameRequestCallback[] = [];
      vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => frames.push(cb));
      vi.spyOn(window, "getComputedStyle").mockReturnValue({
        getPropertyValue: (name: string) => (name === "--duration-glide" ? "520" : "cubic-bezier(0.16, 1, 0.3, 1)"),
      } as CSSStyleDeclaration);
      const first = motion.glideTo(target);
      const second = motion.glideTo(target);
      const now = performance.now();
      frames.splice(0).forEach((cb) => cb(now + 1000));
      await expect(first).resolves.toBe(false);
      frames.splice(0).forEach((cb) => cb(now + 1000));
      await expect(second).resolves.toBe(true);
      expect(target.classList.contains("flash")).toBe(true);
      vi.restoreAllMocks();
    });
  });
});

describe("the Moves and Phone lines of every card (P6)", () => {
  const table = JSON.parse(readFileSync(resolve(UI_ROOT, ".design-sync/moves.json"), "utf8"));
  const config = JSON.parse(readFileSync(resolve(UI_ROOT, ".design-sync/config.json"), "utf8"));
  const cards = Object.keys(config.componentSrcMap);

  it("gives every card a Moves line and a Phone line", () => {
    for (const name of cards) {
      const { moves, phone } = linesFor(name, table);
      expect(moves, `${name} Moves`).toMatch(/\S/);
      expect(phone, `${name} Phone`).toMatch(/\S/);
    }
  });

  it("names only cards that exist", () => {
    for (const name of Object.keys(table.cards)) expect(cards, `moves.json lists ${name}`).toContain(name);
  });

  it("states the still form wherever something moves", () => {
    for (const [name, row] of Object.entries(table.cards) as Array<[string, { moves: string }]>) {
      expect(row.moves, `${name} must say what it does when still`).toMatch(/Still:/);
    }
  });

  it("follows the copy guide: no em dash, no exclamation mark, no keeper's voice", () => {
    const text = JSON.stringify(table.cards) + JSON.stringify(table.defaults);
    expect(text).not.toMatch(/[—!]/);
    expect(text).not.toMatch(/\bI('|’)(ll|m|ve)\b/);
  });

  it("is written into every preview, as the generator would write it", () => {
    const previews = resolve(UI_ROOT, ".design-sync/previews");
    for (const file of readdirSync(previews)) {
      const name = file.replace(/\.tsx$/, "");
      if (!cards.includes(name)) continue;
      const source = readFileSync(resolve(previews, file), "utf8");
      expect(source.startsWith(blockFor(name, table)), `${file} is out of date: run node .design-sync/moves.mjs`).toBe(true);
      expect(withBlock(source, name, table), `${file} is not stable`).toBe(source);
    }
  });

  it("rewrites an old block instead of stacking a second", () => {
    const once = withBlock("import x;\n", "Button", table);
    const edited = once.replace("Presses 3%", "Presses 5%");
    expect(withBlock(edited, "Button", table)).toBe(once);
    expect(once.match(/moves:begin/g)).toHaveLength(1);
  });

  it("MOVES.md has a row for every card", () => {
    const md = readFileSync(resolve(UI_ROOT, ".design-sync/MOVES.md"), "utf8");
    for (const name of cards) expect(md, `MOVES.md lacks ${name}`).toContain(`| ${name} |`);
  });
});

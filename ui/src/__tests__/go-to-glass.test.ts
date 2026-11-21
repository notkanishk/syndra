import { describe, expect, it } from "vitest";

import { goToLightBox } from "@/components/ui/Modal";
import { block, css } from "./cssTokens";

// A-26, Go to as chosen (owner, 21 Nov 2026): its tokens, its one lamp colour, and its lights' limits.

const dark = block(':root,\n  [data-theme="dark"] {');
const light = block('[data-theme="light"] {');

describe("the kinds' colours", () => {
  const turned = /@supports \(color: oklch\(from red l c h\)\) \{\s*:root,\s*\[data-theme\] \{([\s\S]*?)\n    \}/.exec(css)?.[1] ?? "";

  it("are each the accent turned on its own wheel, so no kind is Syndra's own violet (owner: Roles too)", () => {
    for (const kind of ["people", "roles", "pages"]) {
      for (const part of ["", "-text", "-solid"]) {
        const value = new RegExp(`--kind-${kind}${part}: ([^;]+);`).exec(turned)?.[1] ?? "";
        expect(value, `--kind-${kind}${part}`).toMatch(/^oklch\(from var\(--accent(-text|-dense)?\) .* c calc\(h [+-] \d+\)\)$/);
        const turn = Number(/calc\(h ([+-] \d+)\)/.exec(value)?.[1].replace(" ", ""));
        expect(Math.abs(turn), `--kind-${kind}${part} turns`).toBeGreaterThanOrEqual(40);
      }
    }
    // People warm (+44), Pages cool (-46), Roles the aqua beyond Pages (-100): three different turns.
    const turns = ["people", "roles", "pages"].map((kind) => /calc\(h ([+-] \d+)\)/.exec(new RegExp(`--kind-${kind}: ([^;]+);`).exec(turned)![1])![1]);
    expect(turns).toEqual(["+ 44", "- 100", "- 46"]);
  });

  it("fall back to the accent only where a browser cannot turn a colour", () => {
    const plain = /@layer base \{\s*:root,\s*\[data-theme\] \{([\s\S]*?)\n  \}/.exec(css.slice(css.indexOf("── Go to, as chosen")))?.[1] ?? "";
    expect(plain).toMatch(/--kind-roles: var\(--accent\);/);
  });
});

describe("the glass, in both rooms", () => {
  it("is thicker on paper than in the dark room, and solid where less transparency is asked for", () => {
    expect(dark["--glass"]).toBe("78%");
    expect(light["--glass"]).toBe("84%");
    const pane = /\n  \.go-to-pane \{([\s\S]*?)\n  \}\n/.exec(css)?.[1] ?? "";
    expect(pane).toMatch(/background-color: color-mix\(in srgb, var\(--surface-2\) var\(--glass\), transparent\);/);
    expect(pane).toMatch(/@media \(prefers-reduced-transparency: reduce\) \{\s*background-color: var\(--surface-2\);/);
  });
});

describe("one lamp, one colour", () => {
  it("is a registered colour, so a scope's turn is a tint and not a snap; everything the lamp lights reads it", () => {
    expect(css).toMatch(/@property --light \{ syntax: "<color>"; inherits: true; initial-value: transparent; \}/);
    expect(css).toMatch(/\.go-to-scrim \{\s*--light: var\(--accent\);[\s\S]*?transition: --light var\(--duration-tint\) var\(--ease-tint\);/);
    for (const part of [".go-to-scrim", ".go-to-halo", ".go-to-beam", ".go-to-scope-light", ".go-to-pane {"]) {
      const rule = css.slice(css.indexOf(`\n  ${part}`), css.indexOf("\n  }\n", css.indexOf(`\n  ${part}`)));
      expect(rule, part).toMatch(/--pool-c: var\(--light\);/);
    }
  });

  it("draws no light under forced colours", () => {
    expect(css).toMatch(
      /@media \(forced-colors: active\) \{\s*\.go-to-scrim::before,\s*\.go-to-pane::before,\s*\.go-to-wave,\s*\.go-to-beam,\s*\.go-to-halo,\s*\.go-to-scope-light,\s*\.go-to-point \{\s*display: none;/,
    );
  });

  it("breathes as Go to opens only for a visitor who has not asked for less", () => {
    expect(css).toMatch(
      /@media \(forced-colors: none\) and \(prefers-reduced-motion: no-preference\) \{\s*\.go-to-scrim::before \{\s*animation: go-to-light/,
    );
  });
});

describe("the light behind, sized by the window (A-25 §7)", () => {
  const inside = (box: ReturnType<typeof goToLightBox>, view: { width: number; height: number }) =>
    box.x - box.width / 2 >= 0 && box.x + box.width / 2 <= view.width && box.y - box.height / 2 >= 0 && box.y + box.height / 2 <= view.height;

  it("sits on the lamp, or the window's middle when the lamp is near an edge, and is spent before every edge", () => {
    expect(goToLightBox({ x: 406, y: 130 }, { width: 1440, height: 900 })).toMatchObject({ x: 406, width: 812, height: 260 });
    expect(goToLightBox({ x: 42, y: 43 }, { width: 390, height: 844 })).toMatchObject({ x: 195, width: 390, height: 86 });
    for (const view of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
      for (let x = 0; x <= view.width; x += view.width / 8) {
        for (let y = 0; y <= view.height; y += view.height / 8) expect(inside(goToLightBox({ x, y }, view), view)).toBe(true);
      }
    }
  });
});

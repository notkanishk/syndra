import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { SRC, block, colour, css, over, ratio } from "./cssTokens";

// A-25 moment 4, the two repairs that ship first: your place in the rail and the tab bar is marked
// by more than a soft fill, in every room and under forced colours.
const read = (path: string) => readFileSync(resolve(SRC, path), "utf8");
const ramp = block(":root {\n    --violet-600");
const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };

describe("your place in the rail (A-25 moment 4)", () => {
  it("marks it with a dot at 3:1 or more, on its pill and on the rail, in both rooms (WCAG 1.4.11)", () => {
    // The dot's token, read from the rail itself, so the check is of what ships.
    const token = /active \? "bg-([a-z-]+)" : "bg-ink\/20"/.exec(read("components/shell/Sidebar.tsx"))?.[1];
    expect(token).toBe("accent-text");
    // The tab bar's dot is the same token.
    expect(read("components/shell/TouchNav.tsx")).toMatch(/active \? "bg-accent-text" : "bg-ink\/30"/);
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const dot = colour(vars, `--${token}`);
      const rail = colour(vars, "--rail");
      const pill = over(colour(vars, "--accent-soft"), rail);
      for (const [ground, rgba] of [["pill", pill], ["rail", rail]] as const) {
        const r = ratio(dot, rgba);
        if (r < 3) failures.push(`${theme} dot on the ${ground}: ${r.toFixed(2)}`);
      }
      // A-25's bar for low vision, something crisp: the dot at 4.9:1 or better where it sits, on its
      // pill (4.93 dark, 5.76 paper; `bg-accent` was 3.42 and 3.88).
      const onPill = ratio(dot, pill);
      if (onPill < 4.9) failures.push(`${theme} dot on the pill under A-25's 4.9: ${onPill.toFixed(2)}`);
    }
    expect(failures).toEqual([]);
  });

  it("is the system's own selected colours under forced colours, in the rail, the tab bar and the phone's sheet", () => {
    const start = css.indexOf("@utility nav-place {");
    expect(start).toBeGreaterThan(-1);
    const rule = css.slice(start, css.indexOf("\n}\n", start));
    expect(rule).toMatch(
      // The box, its words, no app animation over it, its children inheriting it, and a system focus ring.
      /@media \(forced-colors: active\) \{\s*&\[aria-current="page"\] \{\s*forced-color-adjust: none;\s*background-color: Highlight;\s*color: HighlightText;\s*animation: none;\s*& \* \{\s*background-color: transparent;\s*color: inherit;\s*\}\s*&:focus-visible \{\s*outline-color: CanvasText;\s*\}/,
    );
    // Each current place carries the hook, and is marked the way the rule reads it.
    const places = [
      ["the rail", read("components/shell/Sidebar.tsx"), 1],
      ["the tab bar and the sheet", read("components/shell/TouchNav.tsx"), 2],
    ] as const;
    for (const [name, source, count] of places) {
      const links = [...source.matchAll(/aria-current=\{active \? "page" : undefined\}\s*(?:\/\/[^\n]*\n\s*)*className=\{`([^`$]*)/g)];
      expect(links, name).toHaveLength(count);
      for (const link of links) expect(link[1], name).toMatch(/\bnav-place\b/);
    }
  });
});

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * A-08 §5: the seat is 24px on touch and 16px with a mouse, in the selection bar too (the same
 * `Seat`). The row is the hit area, so a smaller seat must not be a smaller target. jsdom has no
 * layout, so this reads the one place the size is drawn.
 */
const css = readFileSync(join(__dirname, "../../../app/globals.css"), "utf8");
const seat = css.slice(css.indexOf("@utility seat {"), css.indexOf("/* The outcome fading into the slot"));

describe("the selection seat's size", () => {
  it("is 24px by default, and 16px from the desktop breakpoint up", () => {
    expect(seat).toMatch(/^@utility seat \{[^@]*?width: 24px;\s*height: 24px;/);
    expect(seat).toMatch(/@media \(width >= theme\(--breakpoint-desktop\)\) \{\s*width: 16px;\s*height: 16px;/);
  });

  it("keeps the input as big as it was, so the smaller seat is not a smaller target", () => {
    expect(seat).toMatch(/width: 16px;\s*height: 16px;\s*& > input \{\s*inset: -4px;/);
    // Equal specificity, so the later rule wins: drawn before the input's own
    // `inset: 0`, the target shrank to 16px with the seat (seen live).
    expect(seat.indexOf("inset: -4px")).toBeGreaterThan(seat.indexOf("inset: 0"));
  });

  it("scales its tick and dot with the seat rather than drawing them at a fixed size", () => {
    expect(seat).toMatch(/& > span > svg \{\s*width: 50%;\s*height: 50%;/);
  });
});

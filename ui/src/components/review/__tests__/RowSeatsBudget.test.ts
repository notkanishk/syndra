import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { WHAT_COL, WHO_COL } from "@/components/review/RowSeats";
import { NUMBER_COL } from "@/components/ui/Card";
import { CHECK_COL } from "@/components/ui/SelectionBar";

/**
 * The review queues' line budget. jsdom draws no lines, so this sums what a flex line breaks on:
 * each column's width or minimum at a breakpoint, the action seat's min-width and the column gaps,
 * read from globals.css's tokens, against the row the layout check measured. Drift in Select mode
 * came to 760px in a 728px row at 1080, and its seat dropped under every row up to 1110 (19 Nov).
 */
const css = readFileSync(resolve(__dirname, "../../../app/globals.css"), "utf8");
const token = (name: string) => {
  const m = css.match(new RegExp(`${name}:\\s*(\\d+)px`));
  expect(m, name).not.toBeNull();
  return Number(m![1]);
};
const GAP = token("--row-gap");
const SEAT_WIDE = token("--seat-action-wide");

/** The px a column takes at a breakpoint: its width there, else its minimum there. */
function px(cls: string, at: "tablet" | "desktop") {
  const prefixes = at === "desktop" ? ["desktop:", "tablet:", ""] : ["tablet:", ""];
  const parts = cls.split(/\s+/);
  for (const prefix of prefixes) {
    for (const part of parts) {
      if (!part.startsWith(prefix) || part.slice(prefix.length).includes(":")) continue;
      const bare = part.slice(prefix.length);
      const fixed = bare.match(/^w-\[(\d+)px\]$/) ?? bare.match(/^w-\[var\((--[\w-]+)\)\]$/) ?? bare.match(/^w-\((--[\w-]+)\)$/);
      if (fixed) return /^\d+$/.test(fixed[1]) ? Number(fixed[1]) : token(fixed[1]);
      const min = bare.match(/^min-w-\[(\d+)px\]$/);
      if (min) return Number(min[1]);
    }
  }
  throw new Error(`no width or minimum at ${at} on "${cls}"`);
}

const line = (cols: string[], at: "tablet" | "desktop", seat = 0) =>
  cols.reduce((sum, c) => sum + px(c, at), 0) + seat + GAP * (cols.length - (seat ? 0 : 1));

describe("the review queues' columns", () => {
  it("keep Drift on one line in Select mode from a 1080 window: the checkbox, Who, What, Found and the wide seat", () => {
    // 1080 less the rail, the page's inset and the card's, and Drift's 3px edge: measured 728.
    expect(line([CHECK_COL, WHO_COL, WHAT_COL, NUMBER_COL], "desktop", SEAT_WIDE)).toBeLessThanOrEqual(728);
    expect(line([WHO_COL, WHAT_COL, NUMBER_COL], "desktop", SEAT_WIDE)).toBeLessThanOrEqual(728);
  });

  it("keep Expiring's and Unfinished revocations' columns on one line at tablet, What at 90, the seat under them", () => {
    // Their third column shows from desktop. At the narrowest tablet window (720) the row is 374px;
    // with the checkbox (44 there) it fits from 768 (422px).
    expect(px(WHAT_COL, "tablet")).toBe(90);
    expect(line([WHO_COL, WHAT_COL, NUMBER_COL], "tablet")).toBeLessThanOrEqual(374);
    expect(line([CHECK_COL, WHO_COL, WHAT_COL, NUMBER_COL], "tablet")).toBeLessThanOrEqual(422);
  });
});

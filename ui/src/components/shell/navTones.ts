import type { BadgeTone } from "@/lib/nav";

/**
 * A badge's tone, in one place, because the rail and the tab bar are two
 * renderings of one tree and a count that is danger in the rail cannot be
 * accent under a thumb.
 *
 * `accent-dense` rather than `accent`: the label riding on it is small, and the
 * bright fill fails AA below 18.5px. That is the whole reason the dense token
 * exists.
 */
export const BADGE_TONE: Record<BadgeTone, string> = {
  accent: "bg-accent-dense text-accent-ink",
  warn: "bg-warn text-warn-ink",
  danger: "bg-danger text-danger-ink",
};

/** Loudest first. What a collapsed group or a Go-to bar reports as its dot. */
const TONE_RANK: BadgeTone[] = ["danger", "warn", "accent"];

/**
 * The highest tone among destinations that currently want attention.
 *
 * A group that collapses several counted destinations shows this as a dot and
 * never as a sum: three unexplained findings plus eleven expiring grants plus
 * three holds due is not seventeen of anything, and an operator cannot act on
 * a number that spans three different kinds of work.
 */
export function loudestTone(tones: (BadgeTone | undefined)[]): BadgeTone | null {
  for (const rank of TONE_RANK) {
    if (tones.some((tone) => tone === rank)) return rank;
  }
  return null;
}

/** The dot a tone paints when it stands in for a count. */
export const DOT_TONE: Record<BadgeTone, string> = {
  accent: "bg-accent",
  warn: "bg-warn",
  danger: "bg-danger",
};

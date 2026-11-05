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

/**
 * The retry pill a `retries` row carries (lib/nav.ts): a give-up nobody has
 * marked seen outranks a retry under way, because it needs a person and a
 * retry does not. Null when neither, so the pill takes no seat.
 *
 * Its words are Home's and Pending changes' (final verification N2): "Syndra
 * stopped sending", which covers a refusal as well as running out of tries,
 * and a retry is "again on its own" only when the server says its background
 * drain is on (`auto_retry`, from the same indicators read). `again` says
 * whether the pill may wear the "↻" glyph.
 */
export function retryBadge(
  counts: { retrying?: number; gave_up?: number; auto_retry?: boolean } | undefined,
): { count: number; tone: BadgeTone; label: string; again: boolean } | null {
  const gaveUp = Number(counts?.gave_up ?? 0);
  const retrying = Number(counts?.retrying ?? 0);
  const changes = (n: number) => `${n} ${n === 1 ? "change" : "changes"}`;
  if (gaveUp > 0) return { count: gaveUp, tone: "danger", label: `Syndra stopped sending ${changes(gaveUp)}`, again: false };
  if (retrying > 0) {
    const auto = counts?.auto_retry;
    const label =
      auto === true
        ? `Syndra is sending ${changes(retrying)} again on its own`
        : auto === false
          ? `${changes(retrying)} not taken when sent, waiting for you to press Send`
          : `${changes(retrying)} not taken when sent`;
    return { count: retrying, tone: "warn", label: label.charAt(0).toUpperCase() + label.slice(1), again: auto === true };
  }
  return null;
}

/** The dot a tone paints when it stands in for a count. */
export const DOT_TONE: Record<BadgeTone, string> = {
  accent: "bg-accent",
  warn: "bg-warn",
  danger: "bg-danger",
};

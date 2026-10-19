import { formatShortDate } from "@/lib/format";

/**
 * What a settled request to keep a role did to its end, said from the request
 * itself: it remembers the end they had (`ended_before`) and the end approving
 * set (`ends_after`). Both are omitted when null: a null `ended_before` is a
 * role that had lapsed, and a null `ends_after` on an approved request is no
 * end date. A request settled before the ends were recorded carries neither,
 * and nothing is said that isn't known.
 */
export interface KeepEnds {
  ended_before?: string | null;
  ends_after?: string | null;
}

/** An approved keep: "It now ends 18 Jan instead of 18 Dec." */
export function keptEnds({ ended_before, ends_after }: KeepEnds): string | undefined {
  if (ends_after) {
    return ended_before
      ? `It now ends ${formatShortDate(ends_after)} instead of ${formatShortDate(ended_before)}.`
      : `It now ends ${formatShortDate(ends_after)}.`;
  }
  if (ended_before) return `It now has no end date, instead of ending ${formatShortDate(ended_before)}.`;
  return undefined;
}

/** A declined keep: "It still ends 18 Dec." Only an end still ahead can still be one. */
export function stillEnds(ended_before: string | null | undefined): string | undefined {
  if (!ended_before || new Date(ended_before).getTime() <= Date.now()) return undefined;
  return `It still ends ${formatShortDate(ended_before)}.`;
}

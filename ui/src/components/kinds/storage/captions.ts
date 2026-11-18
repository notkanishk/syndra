import { formatBytes } from "@/lib/format";
import { NOTHING_TO_DO } from "@/lib/outcome";

/** What the storage page is for, under its link in the account sheet. */
export const STORAGE_NOTE = "Your password and folders";

/**
 * The totals card's step to Insights, Usage on a storage system (the owner's ruling of 18 Nov 2026,
 * "allow each share here"): a storage word, so it lives with the kind (`kinds/limits.ts` usageStep),
 * and every other kind keeps the usage card's own words. Copy guide section 15.
 */
export const STORAGE_USAGE_STEP = `${NOTHING_TO_DO} Who used each share is in Insights.`;

/** Under this share of the limit there is plenty of room: a judgement, so it is one named number. */
export const PLENTY_BELOW = 0.5;

/**
 * One sentence under the storage meter (A-24 Q18): how much is left, said as a
 * person asks it, from the same read the meter draws: plenty of room under
 * half, then what is left (the meter itself says "nearly full" past 90%). At the
 * limit the card's own "Full." line says it, with its next step, so this is null.
 */
export function roomLine(used: number, quota: number): string | null {
  if (quota <= 0 || used >= quota) return null;
  const left = formatBytes(quota - used);
  return used < quota * PLENTY_BELOW ? "Plenty of room." : `${left} left.`;
}

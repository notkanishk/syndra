import { daysUntil } from "@/lib/format";

/**
 * The academic term boundary access is usually asked for and granted against.
 *
 * A makerspace's natural unit is not 30 days, it is "until term ends" — which
 * is why both the operator's grant dialog and the member's request dialog offer
 * it, and why it lives here rather than inside either of them.
 *
 * The two dates are the institution's, not a computation: 18 May and 18
 * December. Deriving them from anything cleverer would be inventing a calendar.
 */
export function nextTermEnd(now: Date = new Date()): Date {
  const year = now.getFullYear();
  const may = new Date(year, 4, 18);
  const december = new Date(year, 11, 18);
  if (now < may) return may;
  if (now < december) return december;
  return new Date(year + 1, 4, 18);
}

/** Whole days from now to the next term boundary, never less than one. */
export function daysUntilTermEnd(now: Date = new Date()): number {
  return Math.max(1, daysUntil(nextTermEnd(now).toISOString(), now) ?? 1);
}

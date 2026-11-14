import { addDays, civilDay, daysBetween, endOfDayISO } from "@/lib/time";

type TermDates = { name: string; start: string; end: string };

/**
 * The last day of the term holding today, or of the next to start when today
 * falls between two; null when the setting names none. Days are the
 * makerspace's ("YYYY-MM-DD"), compared as text.
 */
export function currentTermEnd(terms: TermDates[], now: Date = new Date()): string | null {
  const today = civilDay(now);
  const sorted = [...terms].sort((a, b) => a.start.localeCompare(b.start));
  const hit =
    sorted.find((t) => t.start <= today && today <= t.end) ??
    sorted.find((t) => t.start > today);
  return hit ? hit.end : null;
}

/**
 * The academic term boundary access is usually asked for and granted against:
 * its last day.
 *
 * A makerspace's natural unit is not 30 days, it is "until term ends" — which
 * is why both the operator's grant dialog and the member's request dialog offer
 * it, and why it lives here rather than inside either of them.
 *
 * The dates come from the term setting. Only when that can't be read (or names
 * no term still to come) does it fall back to 18 May and 18 December, the
 * institution's usual boundaries, rather than offering nothing.
 */
export function nextTermEnd(now: Date = new Date(), terms?: TermDates[]): string {
  const fromSetting = terms?.length ? currentTermEnd(terms, now) : null;
  if (fromSetting) return fromSetting;
  const today = civilDay(now);
  const year = Number(today.slice(0, 4));
  if (today < `${year}-05-18`) return `${year}-05-18`;
  if (today < `${year}-12-18`) return `${year}-12-18`;
  return `${year + 1}-05-18`;
}

/** Calendar days from today to the next term's last day, never less than one. */
export function daysUntilTermEnd(now: Date = new Date(), terms?: TermDates[]): number {
  return Math.max(1, daysBetween(civilDay(now), nextTermEnd(now, terms)));
}

/** What asking to keep a role offers: a way to pick how much longer, and where it lands. */
export interface KeepChoice {
  days: number;
  /** The end it would have, counted from the current end (an instant, for showing). */
  until: string;
  /** The same end as a date, YYYY-MM-DD: what is sent. The backend resolves it to the end of that day in its zone. */
  day: string;
}

/**
 * The two ways to keep a role that ends (A-06 §3). An extension counts from
 * the current end, as the backend does: another month on 18 Dec is 18 Jan.
 * "To the end of next term" is the term after the one holding today (or the
 * next to start, when today falls between two), and is off with its reason
 * when there is none or it isn't later than the current end.
 */
export function keepOptions(
  currentEnd: string,
  terms: Array<{ name: string; start: string; end: string }>,
  now: Date = new Date(),
): { month: KeepChoice; term: KeepChoice | { reason: string } } {
  // "N days" is N calendar days, ending as the Nth does (lib/time.ts), as the backend counts it.
  const endDay = civilDay(new Date(currentEnd));
  const reach = (days: number): KeepChoice => ({ days, until: endOfDayISO(addDays(endDay, days)), day: addDays(endDay, days) });

  const today = civilDay(now);
  const sorted = [...terms].sort((a, b) => a.start.localeCompare(b.start));
  const within = sorted.findIndex((t) => t.start <= today && today <= t.end);
  const next = within >= 0 ? sorted[within + 1] : sorted.find((t) => t.start > today);

  if (!next) return { month: reach(30), term: { reason: "There's no next term set yet." } };
  const days = daysBetween(endDay, next.end);
  if (days < 1) {
    return { month: reach(30), term: { reason: "It already ends after the end of next term." } };
  }
  return { month: reach(30), term: { days, until: endOfDayISO(next.end), day: next.end } };
}

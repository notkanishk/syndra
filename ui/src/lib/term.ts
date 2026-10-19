import { daysUntil } from "@/lib/format";

type TermDates = { name: string; start: string; end: string };

function termDay(ymd: string): Date {
  const [y, m, d] = ymd.split("-").map(Number);
  return new Date(y, m - 1, d);
}

/**
 * The end of the term holding today, or of the next to start when today falls
 * between two; null when the setting names none.
 */
export function currentTermEnd(terms: TermDates[], now: Date = new Date()): Date | null {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const sorted = [...terms].sort((a, b) => a.start.localeCompare(b.start));
  const hit =
    sorted.find((t) => termDay(t.start) <= today && today <= termDay(t.end)) ??
    sorted.find((t) => termDay(t.start) > today);
  return hit ? termDay(hit.end) : null;
}

/**
 * The academic term boundary access is usually asked for and granted against.
 *
 * A makerspace's natural unit is not 30 days, it is "until term ends" — which
 * is why both the operator's grant dialog and the member's request dialog offer
 * it, and why it lives here rather than inside either of them.
 *
 * The dates come from the term setting. Only when that can't be read (or names
 * no term still to come) does it fall back to 18 May and 18 December, the
 * institution's usual boundaries, rather than offering nothing.
 */
export function nextTermEnd(now: Date = new Date(), terms?: TermDates[]): Date {
  const fromSetting = terms?.length ? currentTermEnd(terms, now) : null;
  if (fromSetting) return fromSetting;
  const year = now.getFullYear();
  const may = new Date(year, 4, 18);
  const december = new Date(year, 11, 18);
  if (now < may) return may;
  if (now < december) return december;
  return new Date(year + 1, 4, 18);
}

/** Whole days from now to the next term boundary, never less than one. */
export function daysUntilTermEnd(now: Date = new Date(), terms?: TermDates[]): number {
  return Math.max(1, daysUntil(nextTermEnd(now, terms).toISOString(), now) ?? 1);
}

/** What asking to keep a role offers: a way to pick how much longer, and where it lands. */
export interface KeepChoice {
  days: number;
  /** The end it would have, counted from the current end. */
  until: string;
}

const DAY = 86_400_000;

function localDay(iso: string): Date {
  const d = new Date(iso);
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
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
  const end = new Date(currentEnd);
  const reach = (days: number): KeepChoice => ({
    days,
    until: new Date(end.getTime() + days * DAY).toISOString(),
  });

  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const sorted = [...terms].sort((a, b) => a.start.localeCompare(b.start));
  const within = sorted.findIndex((t) => termDay(t.start) <= today && today <= termDay(t.end));
  const next = within >= 0 ? sorted[within + 1] : sorted.find((t) => termDay(t.start) > today);

  if (!next) return { month: reach(30), term: { reason: "There's no next term set yet." } };
  const days = Math.round((termDay(next.end).getTime() - localDay(currentEnd).getTime()) / DAY);
  if (days < 1) {
    return { month: reach(30), term: { reason: "It already ends after the end of next term." } };
  }
  return { month: reach(30), term: reach(days) };
}

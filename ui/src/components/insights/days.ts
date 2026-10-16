/**
 * Days are local dates ("2026-10-02"), the form the backend reads and writes.
 * Never a Date through a time zone: a day must not move when the clock does.
 */

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sept", "Oct", "Nov", "Dec"];

const pad = (n: number) => String(n).padStart(2, "0");

/** A Date's local day. */
export const dayOf = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const today = () => dayOf(new Date());

export function isDay(s: string | null | undefined): s is string {
  if (!s || !/^\d{4}-\d{2}-\d{2}$/.test(s)) return false;
  const [y, m, d] = s.split("-").map(Number);
  const date = new Date(y, m - 1, d);
  return date.getFullYear() === y && date.getMonth() === m - 1 && date.getDate() === d;
}

const parts = (s: string) => s.split("-").map(Number) as [number, number, number];

/** The day `n` days from `s` (negative for earlier). */
export function addDays(s: string, n: number): string {
  const [y, m, d] = parts(s);
  return dayOf(new Date(y, m - 1, d + n));
}

/** Whole days from `a` to `b`. */
export function daysBetween(a: string, b: string): number {
  const [ay, am, ad] = parts(a);
  const [by, bm, bd] = parts(b);
  return Math.round((Date.UTC(by, bm - 1, bd) - Date.UTC(ay, am - 1, ad)) / 86_400_000);
}

/** "3 Sept": the form a date is spoken in. */
export function dayLabel(s: string): string {
  const [, m, d] = parts(s);
  return `${d} ${MONTHS[m - 1]}`;
}

/** "16 Oct 2026": a date with its year, for a date that may be far from today. */
export const dayLong = (s: string) => `${dayLabel(s)} ${parts(s)[0]}`;

export const monthLabel = (s: string) => MONTHS[parts(s)[1] - 1];
export const isFirstOfMonth = (s: string) => parts(s)[2] === 1;

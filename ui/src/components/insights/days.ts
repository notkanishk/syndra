/**
 * Days are the makerspace's dates ("2026-10-02"), the form the backend reads
 * and writes. Never a Date through a time zone: a day must not move when the
 * clock does. The day arithmetic itself is lib/time.ts's.
 */
import { addDays, civilDay, daysBetween } from "@/lib/time";

export { addDays, daysBetween };

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sept", "Oct", "Nov", "Dec"];

/** An instant's day, in the makerspace's zone. */
export const dayOf = (d: Date) => civilDay(d);
export const today = () => civilDay();

export function isDay(s: string | null | undefined): s is string {
  if (!s || !/^\d{4}-\d{2}-\d{2}$/.test(s)) return false;
  const [y, m, d] = s.split("-").map(Number);
  const date = new Date(y, m - 1, d);
  return date.getFullYear() === y && date.getMonth() === m - 1 && date.getDate() === d;
}

const parts = (s: string) => s.split("-").map(Number) as [number, number, number];

/** "3 Sept": the form a date is spoken in. */
export function dayLabel(s: string): string {
  const [, m, d] = parts(s);
  return `${d} ${MONTHS[m - 1]}`;
}

/** "16 Oct 2026": a date with its year, for a date that may be far from today. */
export const dayLong = (s: string) => `${dayLabel(s)} ${parts(s)[0]}`;

export const monthLabel = (s: string) => MONTHS[parts(s)[1] - 1];
export const isFirstOfMonth = (s: string) => parts(s)[2] === 1;

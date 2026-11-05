import { formatShortDate } from "@/lib/format";

/**
 * Quiet hours: 22:00 up to 05:00, on the reader's own clock (A-18 §4 idea 9).
 * The keeper stops saying "Morning" in the small hours, and a calm Home may say
 * it will keep until morning.
 */
export function isQuietHour(now: Date): boolean {
  const hour = now.getHours();
  return hour >= 22 || hour < 5;
}

/**
 * The time of day, in the reader's own clock: "Morning" from 05:00 to 12:00,
 * "Afternoon" before 17, "Evening" after, and "Evening" again through the quiet
 * hours (22:00 to 05:00), so nobody is told "Morning" at 2 a.m. One keeper
 * greets an operator on Home and a member on My access, so both ask this one
 * function (A-18 §4 idea 2).
 */
export function greeting(now: Date): string {
  const hour = now.getHours();
  if (isQuietHour(now)) return "Evening";
  if (hour < 12) return "Morning";
  if (hour < 17) return "Afternoon";
  return "Evening";
}

/** The one-time welcome (A-18 §4 idea 8), said on the first visit only. */
export const WELCOME = "Welcome to Syndra.";

/** What Syndra is: the first visit's calm rung (A-20 P2), where the ladder has nothing else to say. */
export const ABOUT = "It keeps the makerspace\u2019s list of who can use what, and why.";

/**
 * When the person was last here, in the keeper's words (A-20 P18): "yesterday",
 * a weekday under a week, a date beyond it. `since` goes after "since", `on`
 * after "on" or alone ("on Tuesday", "yesterday"). Null when they were here
 * today or the time is unreadable: "since earlier today" says nothing a
 * refresh would not.
 */
export function lastHere(
  iso: string | null | undefined,
  now: Date,
): { since: string; on: string } | null {
  if (!iso) return null;
  const then = new Date(iso);
  if (Number.isNaN(then.getTime())) return null;
  const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((midnight(now) - midnight(then)) / 86_400_000);
  if (days <= 0) return null;
  if (days === 1) return { since: "yesterday", on: "yesterday" };
  if (days < 7) {
    const weekday = then.toLocaleDateString("en-GB", { weekday: "long" });
    return { since: weekday, on: `on ${weekday}` };
  }
  const date = formatShortDate(iso, now);
  return { since: date, on: `on ${date}` };
}

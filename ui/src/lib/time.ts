/**
 * Time in the makerspace's zone: the one zone every day in Syndra is counted
 * in, on the server and here (owner, 5 Nov 2026: "get everything in one IST
 * time zone"). The browser's own zone is never asked. backend/internal/clock is
 * the same thing on the server, and the two must agree.
 *
 * An instant is absolute; a day is not. So:
 * - a day is "YYYY-MM-DD" (civilDay), never a Date read through a zone;
 * - "until 30 Nov" holds through the whole of 30 Nov (endOfDayISO), and
 *   "for 7 days" is seven calendar days, ending as the 7th one does
 *   (endAfterDaysISO). An end is the last second of its day, so it still reads
 *   as that day wherever it is shown.
 * Never `new Date("YYYY-MM-DD")` (UTC midnight, 05:30 here) or a picker value
 * through `toISOString()`: repoguard-ui's time guard refuses both.
 */

/**
 * The zone is the deployment's `TZ`, read by the backend (clock.Name), the one
 * source: no zone is written here. The root layout reads it from /healthz on
 * the server and hands it to <Providers>, which calls setMakerspaceZone while
 * rendering, before any child formats a time, so the first paint is already in
 * that zone. If it cannot be read (or names no zone) times are shown in UTC and
 * say so (zoneTag), never in a guessed zone.
 */
let zone = "UTC";
let known = false;

/** The zone every date and clock is read in; pass as `{ timeZone: makerspaceZone() }`. */
export const makerspaceZone = (): string => zone;

/** " UTC" while the backend's zone is unknown, so a reader is never left guessing; otherwise empty. */
export const zoneTag = (): string => (known ? "" : " UTC");

const wallClockIn = (timeZone: string) =>
  new Intl.DateTimeFormat("en-US", {
    timeZone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
let wallClock = wallClockIn(zone);

/** Adopt the backend's zone; nothing or a name Intl cannot read means "unknown": UTC, labelled. */
export function setMakerspaceZone(name: string | null | undefined): void {
  let next = "UTC";
  let ok = false;
  if (name) {
    try {
      wallClockIn(name); // throws RangeError on an unknown zone
      next = name;
      ok = true;
    } catch {
      /* unknown: UTC, labelled */
    }
  }
  if (next === zone && ok === known) return;
  zone = next;
  known = ok;
  wallClock = wallClockIn(zone);
}

const pad = (n: number) => String(n).padStart(2, "0");

function wall(at: Date): Record<string, number> {
  const out: Record<string, number> = {};
  for (const p of wallClock.formatToParts(at)) if (p.type !== "literal") out[p.type] = Number(p.value);
  return out;
}

/** The makerspace's calendar day of an instant, "YYYY-MM-DD". */
export function civilDay(at: Date = new Date()): string {
  const w = wall(at);
  return `${w.year}-${pad(w.month)}-${pad(w.day)}`;
}

/** Whether two instants fall on the same makerspace day. */
export const isSameDay = (a: Date, b: Date): boolean => civilDay(a) === civilDay(b);

const parts = (ymd: string) => ymd.split("-").map(Number) as [number, number, number];

/** The day `n` days after `ymd` (negative for earlier). Calendar arithmetic, no zone in it. */
export function addDays(ymd: string, n: number): string {
  const [y, m, d] = parts(ymd);
  const t = new Date(Date.UTC(y, m - 1, d + n));
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}`;
}

/** Whole calendar days from `a` to `b`. */
export function daysBetween(a: string, b: string): number {
  const [ay, am, ad] = parts(a);
  const [by, bm, bd] = parts(b);
  return Math.round((Date.UTC(by, bm - 1, bd) - Date.UTC(ay, am - 1, ad)) / 86_400_000);
}

/** How far the makerspace's wall clock is ahead of UTC at `at`, in ms. */
function offsetMs(at: Date): number {
  const w = wall(at);
  return Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute, w.second) - (at.getTime() - at.getUTCMilliseconds());
}

/** The last second of `ymd` in the makerspace's zone, as an ISO instant. */
export function endOfDayISO(ymd: string): string {
  const [y, m, d] = parts(ymd);
  const asUTC = Date.UTC(y, m - 1, d, 23, 59, 59);
  const guess = asUTC - offsetMs(new Date(asUTC));
  return new Date(asUTC - offsetMs(new Date(guess))).toISOString();
}

/** The last second of the n-th day after today's: 7 days from any time on 5 Nov ends with 12 Nov. */
export function endAfterDaysISO(n: number, now: Date = new Date()): string {
  return endOfDayISO(addDays(civilDay(now), n));
}

/** Calendar days from today to `ymd`, both the makerspace's days. */
export const daysFromToday = (ymd: string, now: Date = new Date()): number => daysBetween(civilDay(now), ymd);

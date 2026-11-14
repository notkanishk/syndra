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
 * The makerspace's zone is the backend's `TZ` (backend/internal/clock.Name),
 * the one source. The root layout reads it from /healthz on the server and
 * hands it to <MakerspaceZone>, which calls setMakerspaceZone while rendering,
 * before any child formats a time: no date is ever drawn in a zone the backend
 * did not name. DEFAULT_ZONE is only what the backend itself falls back to
 * (clock.Default) and what the layout uses if /healthz cannot be read.
 */
export const DEFAULT_ZONE = "Asia/Kolkata";

let zone = DEFAULT_ZONE;

/** The zone every date and clock is read in; pass as `{ timeZone: makerspaceZone() }`. */
export const makerspaceZone = (): string => zone;

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

/** Adopt the backend's zone. An unknown name is refused (kept as is), never half-applied. */
export function setMakerspaceZone(name: string | null | undefined): void {
  if (!name || name === zone) return;
  try {
    wallClock = wallClockIn(name); // throws RangeError on an unknown zone
  } catch {
    return;
  }
  zone = name;
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

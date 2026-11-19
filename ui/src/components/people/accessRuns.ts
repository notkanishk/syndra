import type { RoleHistory, RoleRun } from "@/lib/queries/useUsers";
import { holds, standingWords } from "@/lib/standing";
import { shortDay } from "@/lib/term-dates";
import { civilDay, daysBetween } from "@/lib/time";

/**
 * Access over time (A-24 §9 vivid, Q11): the words and the places on the year's
 * axis, apart from the drawing so both can be read and tested as text. Every day
 * is the makerspace's (`civilDay`), never the browser's.
 */
export const TITLE = "Access over time";
export const NOTE = "This year";
export const CAPTION =
  "Each line runs from the day a role was given to the day it ended. What they hold today is read from Zitadel; the dates are Syndra's records.";
export const CUT_SHORT = "Only the latest changes are drawn, so some earlier starts say not on record.";
export const NOTHING = "No roles given, ended or held this year.";
export const READ_FAILED = "Couldn't read access over time.";

/** The window: this makerspace year, 1 Jan to 31 Dec. `at` is a day's place along it, 0 to 100. */
export function yearOf(today: string) {
  const first = `${today.slice(0, 4)}-01-01`;
  const last = `${today.slice(0, 4)}-12-31`;
  const span = daysBetween(first, last);
  const at = (day: string) => (Math.min(span, Math.max(0, daysBetween(first, day))) / span) * 100;
  return { first, last, at };
}

const dayOf = (iso: string) => civilDay(new Date(iso));

/** Where a run starts: a day in the window, before it, or not on record. */
function startOf(run: RoleRun, first: string): string | "before" | "unknown" {
  if (!run.from) return "unknown";
  const day = dayOf(run.from);
  return day < first ? "before" : day;
}

/** "Not in Zitadel" as the middle of a sentence. */
const lower = (s: string) => s.charAt(0).toLowerCase() + s.slice(1);

/** What today is for an open run: held, or the standing's own words. */
function todayWords(role: RoleHistory): string {
  return role.standing === "held" ? "held today" : lower(standingWords(role.standing, "operator") ?? "");
}

/**
 * The line's short words under its name, of its latest run (the board's): "Since 14 Aug · ends 30
 * Nov", "2 Feb to 14 Aug". A start no record gives is said, never a day put in its place.
 */
export function whenLine(role: RoleHistory, first: string): string {
  const run = role.runs[role.runs.length - 1];
  if (!run) return "";
  const start = startOf(run, first);
  if (run.to) {
    const to = shortDay(dayOf(run.to));
    return start === "unknown" ? `Ended ${to}` : start === "before" ? `Until ${to}` : `${shortDay(start)} to ${to}`;
  }
  const parts = [start === "unknown" ? "Start not on record" : start === "before" ? "Since before this year" : `Since ${shortDay(start)}`];
  if (role.standing !== "held") parts.push(standingWords(role.standing, "operator") ?? "");
  if (role.ends) parts.push(`ends ${shortDay(dayOf(role.ends))}`);
  return parts.join(" · ");
}

/** The whole line as one sentence, every run: "Laser cutting · Operator, given 3 Aug, held today, ends 12 Dec". */
export function lineSentence(role: RoleHistory, first: string, project: string, roleName: string): string {
  const runs = role.runs.map((run) => {
    const start = startOf(run, first);
    const words = [start === "unknown" ? "start not on record" : start === "before" ? "given before this year" : `given ${shortDay(start)}`];
    if (run.to) words.push(`ended ${shortDay(dayOf(run.to))}`);
    else {
      words.push(todayWords(role));
      if (role.ends) words.push(`ends ${shortDay(dayOf(role.ends))}`);
    }
    return words.join(", ");
  });
  return `${project} · ${roleName}, ${runs.join("; ")}`;
}

/** A run's place on the axis, from 0 to 100: `open` it reaches today; `clipped` it began before the window or off the record. */
export interface Placed {
  left: number;
  width: number;
  clipped: boolean;
  /** Ink when held today, grey when it ended (or is open and not held). */
  held: boolean;
}

export function placeRuns(role: RoleHistory, today: string): { runs: Placed[]; ahead: { left: number; width: number } | null } {
  const { first, last, at } = yearOf(today);
  const runs = role.runs.map((run) => {
    const start = startOf(run, first);
    const left = start === "unknown" || start === "before" ? 0 : at(start);
    const right = run.to ? at(dayOf(run.to)) : at(today);
    return { left, width: Math.max(0, right - left), clipped: start === "unknown" || start === "before", held: !run.to && holds(role.standing) };
  });
  const open = role.runs.length > 0 && !role.runs[role.runs.length - 1].to;
  const end = role.ends ? dayOf(role.ends) : null;
  // The time left before an end, dashed amber: only for a role held today that ends, to the year's edge.
  const ahead = open && holds(role.standing) && end && end > today ? { left: at(today), width: at(end > last ? last : end) - at(today) } : null;
  return { runs, ahead };
}

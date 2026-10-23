import { formatShortDate } from "@/lib/format";
import type { Term } from "@/lib/queries/useTerms";

/** The server's own limits (PUT /terms), so the dialog can say them first. */
export const MAX_TERMS = 12;
export const MAX_NAME = 40;

/**
 * A calendar day, "YYYY-MM-DD", as "1 Aug" (the year only when it isn't this
 * year). Read as local midnight: a bare date string parses as UTC, which
 * shows the day before west of Greenwich.
 */
export function shortDay(ymd: string): string {
  return formatShortDate(`${ymd}T00:00:00`);
}

/**
 * The term dates, checked the way the server checks them, keyed the way its
 * 400 keys them (`terms[1].start`), so one place shows either. The server
 * stays the authority; this only says it before sending. An overlap is
 * reported on the later row's start; a shared day counts as one.
 */
export function validateTerms(terms: Term[]): Record<string, string> {
  const errors: Record<string, string> = {};
  if (terms.length < 1) errors["terms"] = "Keep at least one term.";
  if (terms.length > MAX_TERMS) errors["terms"] = `Keep to ${MAX_TERMS} terms or fewer.`;

  const sound: number[] = [];
  terms.forEach((term, i) => {
    const name = term.name.trim();
    if (!name) errors[`terms[${i}].name`] = "Give the term a name.";
    else if (name.length > MAX_NAME) errors[`terms[${i}].name`] = `Keep the name to ${MAX_NAME} characters.`;
    if (!term.start) errors[`terms[${i}].start`] = "Choose a start date.";
    if (!term.end) errors[`terms[${i}].end`] = "Choose an end date.";
    if (term.start && term.end) {
      if (term.start > term.end) errors[`terms[${i}].end`] = "Ends before it starts.";
      else sound.push(i);
    }
  });

  // YYYY-MM-DD sorts as text, so no Date is needed to compare days.
  sound.forEach((j, n) => {
    for (const i of sound.slice(0, n)) {
      const a = terms[i];
      const b = terms[j];
      if (a.start <= b.end && b.start <= a.end && !errors[`terms[${j}].start`]) {
        errors[`terms[${j}].start`] =
          `Overlaps ${a.name.trim() || "another term"}, which runs ${shortDay(a.start)} to ${shortDay(a.end)}. Terms can't share a day.`;
      }
    }
  });
  return errors;
}

/**
 * The field keys of a 400's `details`, leaving out its request id and anything
 * else. The server's reasons are fragments ("overlaps Spring"); each is made a
 * sentence here. The client checks say it first, so this is the second voice.
 */
export function termFieldErrors(details: Record<string, string> | undefined): Record<string, string> {
  return Object.fromEntries(
    Object.entries(details ?? {})
      .filter(([key]) => key.startsWith("terms"))
      .map(([key, why]) => [key, `${why.charAt(0).toUpperCase()}${why.slice(1)}.`]),
  );
}

/**
 * Which term is this one and which is next, by index, so the page can say it
 * after the dates (A-07 §1). "This term" holds today; "Next term" is the first
 * to start after today, so between two terms nothing is this one. Dates are
 * calendar days, compared as text like `validateTerms` does.
 */
export function termStanding(terms: Term[], now: Date = new Date()): (string | null)[] {
  const pad = (n: number) => String(n).padStart(2, "0");
  const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
  let next = -1;
  terms.forEach((t, i) => {
    if (t.start > today && (next < 0 || t.start < terms[next].start)) next = i;
  });
  return terms.map((t, i) =>
    t.start <= today && today <= t.end ? "This term" : i === next ? "Next term" : null,
  );
}

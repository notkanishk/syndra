"use client";

import { countWord } from "@/components/member/headerLine";
import { formatList } from "@/lib/format";
import type { Term } from "@/lib/queries/useTerms";
import { shortDay, termName } from "@/lib/term-dates";
import { civilDay, daysBetween } from "@/lib/time";

/** An access that ends, as Home's Ending soon block reads it (direct, within 14 days). */
export interface Ending {
  user_id: string;
  expires_at?: string | null;
}

/**
 * The term line (A-24 §3.1, Q3), between the date line and the head's dotted
 * lines, as the board draws it: the term's day, a hairline track for the term
 * with the days gone drawn in, today a violet point on it, and an amber mark on
 * each day someone's direct access ends (its day under the pointer), then the
 * term's end. It repeats the Ending
 * soon block in another form, from the same read, so it says nothing that
 * block does not. The words are plain text; only the track is a picture, and
 * its name is the one sentence it adds. Days are the makerspace's (`civilDay`).
 */
export function TermLine({ term, ending, now }: { term: Term; ending: Ending[]; now: Date }) {
  const today = civilDay(now);
  const days = daysBetween(term.start, term.end) + 1;
  // A one-day term has nowhere to travel: its point sits at the start.
  const span = Math.max(1, days - 1);
  const at = (day: string) => `${(Math.min(span, Math.max(0, daysBetween(term.start, day))) / span) * 100}%`;
  const ends = ending.filter((e) => e.expires_at).map((e) => ({ who: e.user_id, day: civilDay(new Date(e.expires_at!)) }));
  const marks = Array.from(new Set(ends.map((e) => e.day).filter((d) => d >= today && d <= term.end))).sort();
  const people = new Set(ends.filter((e) => marks.includes(e.day)).map((e) => e.who)).size;
  const said = marks.length
    ? `Access ends for ${countWord(people)} ${people === 1 ? "person" : "people"} on ${formatList(marks.map(shortDay))}.`
    : undefined;

  // Each mark reads its own day under the pointer (the board: "hover on a mark reads its day").
  const markSaid = (day: string) => {
    const n = new Set(ends.filter((e) => e.day === day).map((e) => e.who)).size;
    return `Access ends for ${countWord(n)} ${n === 1 ? "person" : "people"} on ${shortDay(day)}.`;
  };

  return (
    // As drawn (A-24 frames 2 and 2p): no dot of its own, the words faint, the track the row's free
    // width up to 760px; on a phone the words share the first row and the track takes the full width
    // under them, so "Ends" never wraps alone. measure-exempt: a figure line, not a sentence.
    <div
      data-term-line
      data-figure
      className="grid max-w-[760px] grid-cols-[minmax(0,1fr)_auto] items-center gap-3.5 pb-1.5 pt-3.5 text-small text-faint tablet:grid-cols-[auto_minmax(0,1fr)_auto]"
    >
      <span>{`${termName(term)}, day ${daysBetween(term.start, today) + 1} of ${days}`}</span>
      <span
        data-term-track
        role={said ? "img" : undefined}
        aria-label={said}
        aria-hidden={said ? undefined : true}
        className="relative col-span-2 row-start-2 h-3.5 tablet:col-span-1 tablet:col-start-2 tablet:row-start-1"
      >
        <span className="absolute inset-x-0 top-1/2 h-px bg-line-strong" />
        <span className="absolute left-0 top-1/2 h-px bg-muted" style={{ width: at(today) }} />
        {marks.map((d) => (
          // The 2px mark inside a 10px reach, so a pointer can find it.
          <span key={d} data-ending={d} title={markSaid(d)} className="absolute top-0.5 -ml-[5px] flex h-2.5 w-2.5 justify-center" style={{ left: at(d) }}>
            <span className="h-full w-0.5 rounded-pill bg-warn" />
          </span>
        ))}
        <span
          data-today
          className="absolute top-1/2 size-[7px] -translate-x-1/2 -translate-y-1/2 rounded-pill bg-accent shadow-[0_0_0_4px_var(--accent-soft)]"
          style={{ left: at(today) }}
        />
      </span>
      <span>{`Ends ${shortDay(term.end)}`}</span>
    </div>
  );
}

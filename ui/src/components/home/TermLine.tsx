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
 * The term line (A-24 §3.1, Q3): a hairline for the term holding today, the
 * days gone drawn in, today a violet point, and an amber mark on each day
 * someone's direct access ends. It repeats the Ending soon block in another
 * form, from the same read, so it says nothing that block does not; its words
 * are only the term's day and its end. Days are the makerspace's (`civilDay`).
 */
export function TermLine({ term, ending, now }: { term: Term; ending: Ending[]; now: Date }) {
  const today = civilDay(now);
  const days = daysBetween(term.start, term.end) + 1;
  // A one-day term has nowhere to travel: its point sits at the start.
  const span = Math.max(1, days - 1);
  const at = (day: string) => `${(Math.min(span, Math.max(0, daysBetween(term.start, day))) / span) * 100}%`;
  const name = termName(term);
  const day = `${name}, day ${daysBetween(term.start, today) + 1} of ${days}`;
  const end = `Ends ${shortDay(term.end)}`;
  const ends = ending.filter((e) => e.expires_at).map((e) => ({ who: e.user_id, day: civilDay(new Date(e.expires_at!)) }));
  const marks = Array.from(new Set(ends.map((e) => e.day).filter((d) => d >= today && d <= term.end))).sort();
  const people = new Set(ends.filter((e) => marks.includes(e.day)).map((e) => e.who)).size;
  // The whole line, for a screen reader: an image's own words are not read.
  const said = `${day}. ${end}.${marks.length ? ` Access ends for ${countWord(people)} ${people === 1 ? "person" : "people"} on ${formatList(marks.map(shortDay))}.` : ""}`;

  return (
    <div
      role="img"
      aria-label={said}
      data-figure
      className="measure mt-3.5 grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3.5 gap-y-1.5 text-fine text-faint tablet:grid-cols-[auto_minmax(0,1fr)_auto]"
    >
      <span>{day}</span>
      {/* On a phone the track takes the whole second line (A-24 frame 2p). */}
      <span className="relative col-span-2 row-start-2 h-3.5 tablet:col-span-1 tablet:row-start-1">
        <span className="absolute inset-x-0 top-1/2 h-px bg-line-strong" />
        <span className="absolute left-0 top-1/2 h-px bg-muted" style={{ width: at(today) }} />
        {marks.map((d) => (
          <span key={d} data-ending={d} className="absolute top-0.5 -ml-px h-2.5 w-0.5 rounded-pill bg-warn" style={{ left: at(d) }} />
        ))}
        <span
          data-today
          className="absolute top-1/2 -ml-[3.5px] -mt-[3.5px] size-[7px] rounded-pill bg-accent shadow-[0_0_0_4px_var(--accent-soft)]"
          style={{ left: at(today) }}
        />
      </span>
      <span>{end}</span>
    </div>
  );
}

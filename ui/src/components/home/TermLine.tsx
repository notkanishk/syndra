"use client";

import { StatusDot } from "@/components/ui/Badge";
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
 * The term line (A-24 §3.1, Q3), one of the head's lines: its dot in the same
 * seat as the Claims and Checked lines, the term's day, a hairline track for
 * the term with the days gone drawn in, today on it, and an amber mark on each
 * day someone's direct access ends, then the term's end. It repeats the Ending
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

  return (
    <div data-term-line data-figure className="flex min-h-6 flex-wrap items-center gap-x-2 gap-y-1 pt-1 text-small text-muted">
      <StatusDot tone="accent" />
      <span>{`${termName(term)}, day ${daysBetween(term.start, today) + 1} of ${days}`}</span>
      <span
        role={said ? "img" : undefined}
        aria-label={said}
        aria-hidden={said ? undefined : true}
        className="relative h-3.5 min-w-24 max-w-80 flex-1"
      >
        <span className="absolute inset-x-0 top-1/2 h-[3px] -translate-y-1/2 rounded-pill bg-tint-3" />
        <span className="absolute left-0 top-1/2 h-[3px] -translate-y-1/2 rounded-pill bg-line-strong" style={{ width: at(today) }} />
        {marks.map((d) => (
          <span key={d} data-ending={d} className="absolute top-0.5 -ml-px h-2.5 w-0.5 rounded-pill bg-warn" style={{ left: at(d) }} />
        ))}
        <span data-today className="absolute top-1/2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-pill bg-accent" style={{ left: at(today) }} />
      </span>
      <span className="text-faint">{`Ends ${shortDay(term.end)}`}</span>
    </div>
  );
}

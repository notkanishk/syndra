"use client";

import { useState, type KeyboardEvent } from "react";

import { countWord } from "@/components/member/headerLine";
import { formatList } from "@/lib/format";
import type { Term } from "@/lib/queries/useTerms";
import { shortDay, termName } from "@/lib/term-dates";
import { Crossfade } from "@/lib/motion";
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
 * each day someone's direct access ends (hover or focus reads its day), then the
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
  // One sentence for any set of days: the track's name (every mark) and the readout (one), people once each.
  const endsOn = (on: string[]) => {
    const n = new Set(ends.filter((e) => on.includes(e.day)).map((e) => e.who)).size;
    return `Access ends for ${countWord(n)} ${n === 1 ? "person" : "people"} on ${formatList(on.map(shortDay))}.`;
  };
  const said = marks.length ? endsOn(marks) : undefined;
  const termDay = `${termName(term)}, day ${daysBetween(term.start, today) + 1} of ${days}`;

  // Hover or focus reads a mark's day, as the usage band does (A-24 frame 2): the track is one focusable
  // image, ← and → move between marks, and the words go to the line's readout, the term's day's seat.
  const [read, setRead] = useState<number | null>(null);
  const reading = read !== null && read < marks.length ? read : null;
  const readout = reading === null ? termDay : endsOn([marks[reading]]);
  const keys = (e: KeyboardEvent) => {
    const by = e.key === "ArrowLeft" ? -1 : e.key === "ArrowRight" ? 1 : 0;
    if (!by || !marks.length) return;
    e.preventDefault();
    setRead(Math.min(marks.length - 1, Math.max(0, (reading ?? 0) + by)));
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
      {/* Every reading in one cell, the unread ones unseen, so the seat is as wide as the widest and
          reading a mark never moves the track under the pointer. */}
      <span data-term-readout className="grid">
        {marks.map((d) => (
          <span key={d} aria-hidden className="invisible [grid-area:1/1]">
            {endsOn([d])}
          </span>
        ))}
        <span aria-live="polite" className={`[grid-area:1/1] ${reading === null ? "" : "text-muted"}`}>
          <Crossfade id={readout} inline>
            {readout}
          </Crossfade>
        </span>
      </span>
      <span
        data-term-track
        role={said ? "img" : undefined}
        aria-label={said}
        aria-hidden={said ? undefined : true}
        tabIndex={said ? 0 : undefined}
        onFocus={() => setRead((r) => r ?? 0)}
        onBlur={() => setRead(null)}
        onKeyDown={keys}
        onPointerLeave={() => setRead(null)}
        className="relative col-span-2 row-start-2 h-3.5 rounded-inner tablet:col-span-1 tablet:col-start-2 tablet:row-start-1"
      >
        <span className="absolute inset-x-0 top-1/2 h-px bg-line-strong" />
        <span className="absolute left-0 top-1/2 h-px bg-muted" style={{ width: at(today) }} />
        {marks.map((d, i) => (
          // The 2px mark inside a 10px reach, so a pointer can find it; the one being read is tinted, colour only.
          <span
            key={d}
            data-ending={d}
            data-read={reading === i ? "" : undefined}
            onPointerEnter={() => setRead(i)}
            className={`absolute top-0.5 -ml-[5px] flex h-2.5 w-2.5 justify-center rounded-inner ${reading === i ? "bg-tint-3" : ""}`}
            style={{ left: at(d) }}
          >
            <span className={`h-full w-0.5 rounded-pill bg-warn`} />
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

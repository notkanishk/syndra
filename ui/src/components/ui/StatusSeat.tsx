"use client";

import { STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Crossfade } from "@/lib/motion";

/**
 * A status said in a seat: a dot, the word, and one faint note. When what it
 * says changes in place (Motion v2 S4) the dot stays where it is and changes
 * colour on tint, and the words crossfade in a seat as tall as the taller of
 * the two. It never blooms: a bloom is for something that went well because of
 * you, and that is `ActionOutcome`'s seat.
 *
 * The dot's column is `--dot-indent` wide (the dot and its gap), so the word,
 * the note and the `next` step all start at one edge: a line under a status
 * dot starts at the word, wherever it is drawn (D-1 §4, rule 5).
 */
export function StatusSeat({
  tone,
  word,
  note,
  next,
}: {
  tone: StatusTone;
  word: string;
  note?: string;
  /** The step, under the words, in the same column. */
  next?: React.ReactNode;
}) {
  return (
    <span className="grid grid-cols-[var(--dot-indent)_1fr] items-start gap-y-1">
      <StatusDot tone={tone} className="mt-[7px]" />
      <Crossfade id={`${word}\n${note ?? ""}`}>
        <span className="flex flex-col gap-1">
          <span className={`whitespace-nowrap text-small font-semibold ${STATUS_TONE[tone].label}`}>
            {word}
          </span>
          {note && <span className="text-fine leading-[1.45] text-faint">{note}</span>}
        </span>
      </Crossfade>
      {next && <span className="col-start-2">{next}</span>}
    </span>
  );
}

/**
 * A line its status dot leads, as a page's head says it: My access's line and
 * the line that knows on a person's page (A-24 frame 18), one `HeaderLine`
 * drawn one way. The dot's seat is always there, unseen while there is no tone
 * (the read is out), so the words never shift when it lands; the words
 * crossfade when they change.
 */
export function DotLine({ tone, text, className = "" }: { tone: StatusTone | "none"; text: string; className?: string }) {
  return (
    <span data-dot-line className={`flex items-center gap-2 ${className}`}>
      <StatusDot tone={tone === "none" ? "neutral" : tone} className={tone === "none" ? "invisible" : ""} />
      <Crossfade id={text}>{text}</Crossfade>
    </span>
  );
}

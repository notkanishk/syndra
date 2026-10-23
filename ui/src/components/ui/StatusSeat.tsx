"use client";

import { STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Crossfade } from "@/components/ui/Crossfade";

/**
 * A status said in a seat: a dot, the word, and one faint note. When what it
 * says changes in place (Motion v2 S4) the dot stays where it is and changes
 * colour on tint, and the words crossfade in a seat as tall as the taller of
 * the two. It never blooms: a bloom is for something that went well because of
 * you, and that is `ActionOutcome`'s seat.
 */
export function StatusSeat({ tone, word, note }: { tone: StatusTone; word: string; note?: string }) {
  return (
    <span className="grid grid-cols-[auto_1fr] items-start gap-x-2">
      <StatusDot tone={tone} className="mt-[7px]" />
      <Crossfade id={`${word}\n${note ?? ""}`}>
        <span className="flex flex-col gap-1">
          <span className={`whitespace-nowrap text-[13.5px] font-semibold ${STATUS_TONE[tone].label}`}>
            {word}
          </span>
          {note && <span className="text-[12.5px] leading-[1.45] text-faint">{note}</span>}
        </span>
      </Crossfade>
    </span>
  );
}

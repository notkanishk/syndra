"use client";

import { FieldHint, Input } from "@/components/ui/Input";
import { Segmented } from "@/components/ui/Select";
import { formatShortDate } from "@/lib/format";

export type Until = "ends" | "review";

/**
 * "Ends on" or "Review on", and the day: the one chooser every own-value hold
 * is bounded with (a limit given in the dialog, a limit adopted from a
 * finding). The explanation of each choice and the words for its outcome live
 * here and nowhere else, so what the hint promises and what the result says
 * cannot drift apart.
 *
 * A review date does NOT end the value: it stays until somebody keeps or ends
 * it, and surfaces under holds due for review on that day.
 */
export function UntilChoice({
  until,
  onUntil,
  date,
  onDate,
  comesBack,
  endsNote,
}: {
  until: Until;
  onUntil: (u: Until) => void;
  date: string;
  onDate: (d: string) => void;
  /** What returns when it ends, in words ("their roles' up to 100 GB"). */
  comesBack: string;
  /** Anything more a caller must say about the end of this kind of value. */
  endsNote?: string;
}) {
  return (
    <>
      <span className="flex flex-wrap items-center gap-3">
        <Segmented
          label="Until"
          value={until}
          onChange={onUntil}
          options={[
            { value: "ends", label: "Ends on" },
            { value: "review", label: "Review on" },
          ]}
        />
        <span className="w-[170px]">
          <Input aria-label={until === "ends" ? "Ends on" : "Review on"} type="date" value={date} onChange={(e) => onDate(e.target.value)} />
        </span>
      </span>
      <FieldHint>
        {until === "ends"
          ? `On that day ${comesBack} comes back by itself.${endsNote ? ` ${endsNote}` : ""}`
          : "It stays until you keep it or end it. On that day it lists under holds due for review."}
      </FieldHint>
    </>
  );
}

/** What happens and when, to finish a sentence that begins "It": the outcome's half of the same words. */
export function untilWords(until: Until, day: string): string {
  return until === "ends"
    ? `ends on ${formatShortDate(day)}`
    : `stays until somebody keeps or ends it, and lists under holds due for review on ${formatShortDate(day)}`;
}

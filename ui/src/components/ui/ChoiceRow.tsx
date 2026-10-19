import type { ReactNode } from "react";

import { Seat } from "@/components/ui/Choice";

/**
 * One choice in a picker (proposed/SelectionSeat.md, route-bundles-v2 3a):
 * the row is the control. A round seat at the start, the name, then what it
 * is. Chosen rows take the violet tint and edge of a chosen answer; a locked
 * row is chosen by something elsewhere and says where; a blocked row can't be
 * chosen and says why, in place of its description.
 */
export function ChoiceRow({
  selected,
  onToggle,
  single = false,
  name,
  locked = false,
  lockedNote,
  blocked = false,
  blockedReason,
  disabled = false,
  title,
  meta,
  description,
  aside,
  note: seatNote,
  outlined = false,
}: {
  selected: boolean;
  onToggle: () => void;
  /** One of many: the seat holds a dot, and `name` groups the radios. */
  single?: boolean;
  name?: string;
  locked?: boolean;
  lockedNote?: string;
  blocked?: boolean;
  blockedReason?: string;
  disabled?: boolean;
  title: ReactNode;
  /** Faint, after the title: "v3 · 4 roles". */
  meta?: ReactNode;
  description?: ReactNode;
  /** The row's right seat, e.g. a Waits-for-you mark with words. */
  aside?: ReactNode;
  /**
   * What the row says about itself on the right (max 170, right-aligned, 12.5px):
   * "You have this", "Yours until 18 Dec". The description stays under the name.
   */
  note?: ReactNode;
  /** Draws an edge so the eye lands here: the row a link named that can't be chosen. */
  outlined?: boolean;
}) {
  const inert = locked || blocked || disabled;
  const chosen = selected && !locked && !blocked;
  const note = blocked ? blockedReason : locked ? lockedNote : undefined;

  return (
    <label
      className={`flex min-h-[44px] items-start gap-3 rounded-inner border px-3.5 py-2.5 motion-tint ${
        chosen ? "border-accent-line bg-accent-soft" : outlined ? "border-line-strong" : "border-transparent"
      } ${inert ? "" : "cursor-pointer hover:bg-[var(--hover)]"} ${chosen ? "hover:bg-accent-soft" : ""}`}
    >
      <Seat
        single={single}
        name={name}
        locked={locked}
        checked={chosen}
        disabled={blocked || disabled}
        onChange={onToggle}
        className={`mt-px ${blocked ? "opacity-60" : ""}`}
      />
      <span className={`flex min-w-0 flex-1 flex-col gap-0.5 ${blocked ? "opacity-60" : ""}`}>
        <span className="text-[14.5px] leading-[1.4]">
          <span className={`font-semibold ${locked ? "text-muted" : "text-ink"}`}>{title}</span>
          {meta && <span className="text-[13px] text-faint"> · {meta}</span>}
        </span>
        {(note ?? description) && (
          <span className="text-[13px] leading-[1.45] text-muted [text-wrap:pretty]">
            {note ?? description}
          </span>
        )}
      </span>
      {seatNote && (
        <span className="mt-0.5 flex max-w-[170px] flex-none flex-col items-end gap-0.5 text-right text-[12.5px] leading-[1.4] text-muted">
          {seatNote}
        </span>
      )}
      {aside && <span className="mt-0.5 flex-none">{aside}</span>}
    </label>
  );
}

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
}) {
  const inert = locked || blocked || disabled;
  const chosen = selected && !locked && !blocked;
  const note = blocked ? blockedReason : locked ? lockedNote : undefined;

  return (
    <label
      className={`flex min-h-[44px] items-start gap-3 rounded-inner border px-3.5 py-2.5 motion-tint ${
        chosen ? "border-accent-line bg-accent-soft" : "border-transparent"
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
      {aside && <span className="mt-0.5 flex-none">{aside}</span>}
    </label>
  );
}

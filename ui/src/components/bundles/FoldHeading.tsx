"use client";

import { FoldChevron } from "@/components/ui/FoldChevron";

/** A place or a group, folded: a round chevron, its name, and what is in it. */
export function FoldHeading({
  open,
  onClick,
  title,
  count,
  picked,
}: {
  open: boolean;
  onClick: () => void;
  title: string;
  count: string;
  picked: boolean;
}) {
  return (
    <button
      type="button"
      aria-expanded={open}
      onClick={onClick}
      className="flex min-h-[44px] w-full items-center gap-3 px-3 py-2 text-left hover-tint"
    >
      <FoldChevron open={open} />
      <span className="min-w-0 flex-1 text-[14.5px] font-semibold">{title}</span>
      <span className={`text-[13px] ${picked ? "font-semibold text-accent-text" : "text-faint"}`}>{count}</span>
    </button>
  );
}

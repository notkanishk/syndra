"use client";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { NextStep } from "@/components/ui/NextStep";

/**
 * A place or a group, folded: a round chevron, its name, and what is in it
 * (A-21 §2.9, proposed/FoldHeading.md). A plan's folds add a status dot, a
 * faint line beneath ("Because Trained in Laser Lab always waits for you."),
 * and after Apply the wait fold's word is the link to Pending changes itself,
 * drawn by NextStep. The caller wraps the heading and its rows in one
 * `role="group"` labelled "{title}, {count}".
 */
export function FoldHeading({
  open,
  onClick,
  title,
  count,
  picked = false,
  dot,
  bloom = false,
  note,
  href,
}: {
  open: boolean;
  onClick: () => void;
  title: string;
  count: string;
  picked?: boolean;
  dot?: StatusTone;
  /** The dot blooms once: only for something that just went well. */
  bloom?: boolean;
  note?: string;
  /** The word is a link (NextStep): the chevron alone folds, so a link never sits inside a button. */
  href?: string;
}) {
  const tally = <span className={`text-[13px] ${picked ? "font-semibold text-accent-text" : "text-faint"}`}>{count}</span>;
  const beneath = note && <span className="block pb-2 pl-[52px] pr-3 text-[12.5px] leading-[1.45] text-faint">{note}</span>;
  if (href) {
    return (
      <>
        <span className="flex min-h-[var(--tap)] w-full items-center gap-3 px-3 py-2">
          <button
            type="button"
            aria-expanded={open}
            aria-label={title}
            onClick={onClick}
            className="-m-2 flex size-[var(--tap)] flex-none items-center justify-center rounded-pill hover-tint"
          >
            <FoldChevron open={open} />
          </button>
          <span className="min-w-0 flex-1">
            <NextStep href={href} />
          </span>
          {tally}
        </span>
        {beneath}
      </>
    );
  }
  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        onClick={onClick}
        className="flex min-h-[var(--tap)] w-full items-center gap-3 px-3 py-2 text-left hover-tint"
      >
        <FoldChevron open={open} />
        <span className="flex min-w-0 flex-1 items-center gap-2 text-[14.5px] font-semibold">
          {dot && <StatusDot tone={dot} bloom={bloom} />}
          {title}
        </span>
        {tally}
      </button>
      {beneath}
    </>
  );
}

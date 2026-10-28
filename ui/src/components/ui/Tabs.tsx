"use client";

import React from "react";

import { PILL } from "@/components/ui/Button";
import { GlideIndicator, useGlideIndicator } from "@/components/ui/GlideIndicator";

/**
 * The page-level tab row: two or three views of one subject, switched in place.
 *
 * It exists because there were two hand-rolled copies of it — the person page
 * and the drift queue — and they had already drifted from each other in the
 * ways copies always do. Same 44px floor spelled two ways (`min-h-11` and
 * `min-h-[44px]`), and vertical padding applied always in one and only above
 * the desktop breakpoint in the other, so the same control was a different
 * height on a phone depending on which screen you were on.
 *
 * Distinct from `FilterPills`, which narrows a list and is a `radiogroup`.
 * This one changes what the page is showing, so each tab is the current page or
 * it is not — `aria-current="page"`, the same contract the navigation rail uses.
 *
 * Its box is `Button` size="md" minus the border, which is also `FilterPills`'
 * box: an action, a filter and a tab sit within a hundred pixels of each other
 * on the drift queue, and they were 13.5px, 13px and 14.5px respectively. Three
 * sizes there read as three accidents rather than three kinds of control — the
 * difference between them is carried by shape and fill. The current tab is
 * marked by an underline that glides to the choice (S10, `GlideIndicator`),
 * and its content swaps in place with no slide.
 *
 * `label` is a node rather than a string because a tab may carry its own count,
 * and a count beside a tab label is the one place a number belongs inside a
 * control instead of beside it.
 */
export function Tabs<T extends string>({
  options,
  value,
  onSelect,
  label,
  className = "",
}: {
  options: Array<{ value: T; label: React.ReactNode }>;
  value: T;
  onSelect: (next: T) => void;
  /** Names the group for a screen reader — "Views of this person's access". */
  label: string;
  className?: string;
}) {
  const { ref, box } = useGlideIndicator(value);
  return (
    <div
      ref={ref}
      // A group, so the label is announced: a bare div's aria-label is not.
      // The tabs stay links-in-spirit (`aria-current`), not an ARIA tablist,
      // because each one changes what the page shows rather than a panel's.
      role="group"
      aria-label={label}
      className={`relative flex flex-wrap items-center gap-2 border-b border-line ${className}`}
    >
      <GlideIndicator box={box} kind="underline" className="bg-accent" />
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => onSelect(option.value)}
            aria-current={active ? "page" : undefined}
            className={`relative rounded-pill control ${PILL.md} ${
              active ? "font-semibold text-ink" : "text-muted hover:text-ink"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

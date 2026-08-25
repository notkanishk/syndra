"use client";

import React from "react";

import { PILL } from "@/components/ui/Button";

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
 * difference between them is carried by shape and fill.
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
  return (
    <div aria-label={label} className={`flex flex-wrap items-center gap-2 ${className}`}>
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => onSelect(option.value)}
            aria-current={active ? "page" : undefined}
            className={`rounded-pill motion-tint ${PILL.md} ${
              active ? "bg-tint-3 font-semibold text-ink" : "text-muted hover:text-ink"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

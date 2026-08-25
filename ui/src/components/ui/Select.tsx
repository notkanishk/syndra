"use client";

import React from "react";

import { PILL } from "@/components/ui/Button";

/**
 * Native select, styled to match Input. Native rather than a custom listbox:
 * it is keyboard-accessible for free, it works on touch, and nothing here
 * needs multi-select or async search.
 *
 * `emphasis` gives the field an accent border — used for the field that is
 * currently the decision on the panel (E5's role select).
 */
export const Select = React.forwardRef<
  HTMLSelectElement,
  React.SelectHTMLAttributes<HTMLSelectElement> & { emphasis?: boolean }
>(function Select({ className = "", emphasis = false, children, ...props }, ref) {
  return (
    <select
      ref={ref}
      className={`block w-full appearance-none rounded-inner border bg-transparent px-[15px] py-3 text-[15px] text-ink motion-tint disabled:cursor-not-allowed disabled:text-faint ${
        emphasis ? "border-accent-line" : "border-line-strong"
      } ${className}`}
      {...props}
    >
      {children}
    </select>
  );
});

/**
 * A segmented pill — two or more mutually exclusive options, all legible at
 * once. Used for the token format, the access-map depth, and the source
 * filters. Never a dropdown where the options are this few: a dropdown hides
 * the alternative behind a click.
 */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  size = "md",
  label,
}: {
  options: Array<{ value: T; label: string }>;
  value: T;
  onChange: (next: T) => void;
  size?: "sm" | "md";
  label: string;
}) {
  // Segments are controls, and on touch they carry the same floor every other
  // control does. The compact padding returns above the desktop breakpoint,
  // where a segmented control sits in a dense toolbar.
  const pad =
    size === "sm"
      ? "min-h-[44px] px-3 text-[12.5px] desktop:min-h-0 desktop:py-1"
      : "min-h-[44px] px-4 text-[13px] desktop:min-h-0 desktop:py-1.5";
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-pill bg-tint-2 p-[3px]">
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(option.value)}
            className={`rounded-pill font-semibold motion-tint ${pad} ${
              active ? "bg-accent-dense text-accent-ink" : "text-muted hover:text-ink"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

/** Filter pills — like Segmented, but neutral, because a filter is not an action. */
export function FilterPills<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: Array<{ value: T; label: string }>;
  value: T;
  onChange: (next: T) => void;
  label: string;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap items-center gap-1">
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(option.value)}
            // The same box as `Button` size="md" minus its border: 44px floor
            // through the tablet range, then px-4 / 13.5px / py-[7px]. The
            // three pill controls — an action, a filter and a tab — were
            // 13.5px, 13px and 14.5px, and the drift queue stacks all three
            // within 120px of each other, where three sizes read as three
            // accidents. Shape and fill carry the difference between them now,
            // which is what was meant to carry it.
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

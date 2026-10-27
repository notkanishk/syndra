"use client";

import React from "react";

import { PILL } from "@/components/ui/Button";
import { GlideIndicator, useGlideIndicator } from "@/components/ui/GlideIndicator";

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
      className={`block w-full appearance-none rounded-inner border bg-transparent px-[15px] py-3 text-[15px] text-ink motion-tint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${
        emphasis ? "border-accent-line" : "border-line-strong"
      } ${className}`}
      {...props}
    >
      {children}
    </select>
  );
});

/**
 * The WAI-ARIA radio-group keyboard pattern, shared by the two radiogroups
 * below: one tab stop (the checked option, or the first enabled one when none
 * is), arrows move to the next enabled option and select it, Home/End jump to
 * the ends. Both are controlled, so selecting is `onChange` and the focus
 * follows by hand.
 */
function radioGroup<T extends string>(
  options: Array<{ value: T; disabled?: boolean }>,
  value: T,
  onChange: (next: T) => void,
) {
  const enabled = options.filter((option) => !option.disabled);
  const stop = enabled.find((option) => option.value === value) ?? enabled[0];
  return {
    tabIndexOf: (optionValue: T) => (optionValue === stop?.value ? 0 : -1),
    onKeyDown(event: React.KeyboardEvent<HTMLElement>) {
      const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
      if (!step && event.key !== "Home" && event.key !== "End") return;
      if (enabled.length === 0) return;
      const at = enabled.findIndex((option) => option.value === value);
      // Nothing checked yet: forward starts at the first option, back at the last.
      const toFirst = event.key === "Home" || (at < 0 && step === 1);
      const toLast = event.key === "End" || (at < 0 && step === -1);
      const next = toFirst
        ? enabled[0]
        : toLast
          ? enabled[enabled.length - 1]
          : enabled[(at + step! + enabled.length) % enabled.length];
      event.preventDefault();
      onChange(next.value);
      event.currentTarget.querySelectorAll<HTMLElement>('[role="radio"]')[options.indexOf(next)]?.focus();
    },
  };
}

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
  describedBy,
  fill = false,
}: {
  /** `title` is the segment's own hover text, for a choice that needs a sentence beside its word. */
  options: Array<{ value: T; label: string; title?: string; disabled?: boolean }>;
  value: T;
  onChange: (next: T) => void;
  size?: "sm" | "md";
  label: string;
  /** The id of a sentence that explains the whole choice. */
  describedBy?: string;
  /** Spans its container, each segment an equal share: the phone sheet's form. */
  fill?: boolean;
}) {
  // Segments are controls, and on touch they carry the same floor every other
  // control does. The compact padding returns above the desktop breakpoint,
  // where a segmented control sits in a dense toolbar.
  const pad =
    size === "sm"
      ? "min-h-[44px] px-3 text-[12.5px] desktop:min-h-0 desktop:py-1"
      : "min-h-[44px] px-4 text-[13px] desktop:min-h-0 desktop:py-1.5";
  const { ref, box } = useGlideIndicator(value);
  const keys = radioGroup(options, value, onChange);
  return (
    <div
      ref={ref}
      role="radiogroup"
      aria-label={label}
      aria-describedby={describedBy}
      onKeyDown={keys.onKeyDown}
      className={`${fill ? "flex w-full" : "inline-flex"} relative rounded-pill bg-tint-2 p-[3px]`}
    >
      <GlideIndicator box={box} kind="fill" className="bg-accent-dense" />
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            tabIndex={keys.tabIndexOf(option.value)}
            aria-checked={active}
            title={option.title}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
            className={`relative rounded-pill font-semibold control ${fill ? "flex-1" : ""} ${pad} ${
              active ? `${box ? "" : "bg-accent-dense "}text-accent-ink` : "text-muted hover:text-ink"
            } ${option.disabled ? "cursor-not-allowed opacity-50" : ""}`}
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
  /** A disabled option stays where it is, with `title` saying why. */
  options: Array<{ value: T; label: string; disabled?: boolean; title?: string }>;
  value: T;
  onChange: (next: T) => void;
  label: string;
}) {
  const { ref, box } = useGlideIndicator(value);
  const keys = radioGroup(options, value, onChange);
  return (
    <div ref={ref} role="radiogroup" aria-label={label} onKeyDown={keys.onKeyDown} className="relative flex flex-wrap items-center gap-1">
      <GlideIndicator box={box} kind="fill" className="bg-tint-3" />
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            tabIndex={keys.tabIndexOf(option.value)}
            aria-checked={active}
            disabled={option.disabled}
            title={option.title}
            onClick={() => onChange(option.value)}
            // The same box as `Button` size="md" minus its border: 44px floor
            // through the tablet range, then px-4 / 13.5px / py-[7px]. The
            // three pill controls — an action, a filter and a tab — were
            // 13.5px, 13px and 14.5px, and the drift queue stacks all three
            // within 120px of each other, where three sizes read as three
            // accidents. Shape and fill carry the difference between them now,
            // which is what was meant to carry it.
            className={`relative rounded-pill control ${PILL.md} ${
              active ? `${box ? "" : "bg-tint-3 "}font-semibold text-ink` : "text-muted hover:text-ink"
            } disabled:cursor-not-allowed disabled:text-faint disabled:hover:text-faint`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

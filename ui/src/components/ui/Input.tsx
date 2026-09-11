"use client";

import React from "react";

/**
 * Text input. 14px radius rather than a pill: a field the operator types a
 * claim name or a date into should read as a container, not as a control.
 */
export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className = "", ...props }, ref) {
    return (
      <input
        ref={ref}
        className={`block w-full rounded-inner border border-line-strong bg-transparent px-[15px] py-3 text-[15px] text-ink motion-tint placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
        {...props}
      />
    );
  },
);

/**
 * A short reason, two rows tall rather than one line stretched to a wall.
 * `rows={2}` is a default, not a floor — a caller with a longer reason to
 * capture can still pass a bigger one.
 */
export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className = "", rows = 2, ...props }, ref) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      className={`block w-full resize-y rounded-inner border border-line-strong bg-transparent px-[15px] py-3 text-[15px] text-ink motion-tint placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
      {...props}
    />
  );
});

/** A field label — 12.5px/600, quiet, sitting directly above its control. */
export function FieldLabel({
  children,
  htmlFor,
  // For a field that is a GROUP of controls rather than one input — a role
  // picker, a set of choices. `htmlFor` cannot point at a div, so the div
  // points back at this instead.
  id,
  className = "",
}: {
  children: React.ReactNode;
  htmlFor?: string;
  id?: string;
  className?: string;
}) {
  return (
    <label
      id={id}
      htmlFor={htmlFor}
      className={`mb-[7px] block text-[12.5px] font-semibold text-faint ${className}`}
    >
      {children}
    </label>
  );
}

/** The helper line beneath a field — plain language, never a spec. */
export function FieldHint({ children }: { children: React.ReactNode }) {
  return <p className="mt-[7px] text-[13px] text-faint">{children}</p>;
}

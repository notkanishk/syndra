"use client";

import React, { useRef } from "react";

import { TINT_MS, useExitPhase } from "@/lib/motion";

/**
 * A field at the sm control's height (A-16 §4): 34px with a mouse, the 44px
 * floor below desktop, as `PILL.sm` is for a button. For fields that sit in a
 * card's rows (the claims editor), where the 46px field was the outsized one.
 */
export const DENSE_FIELD = "min-h-[var(--tap)] py-[5px] text-[13.5px] desktop:min-h-0";

/**
 * Text input. 14px radius rather than a pill: a field the operator types a
 * claim name or a date into should read as a container, not as a control.
 */
export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement> & { dense?: boolean }>(
  function Input({ className = "", dense = false, ...props }, ref) {
    return (
      <input
        ref={ref}
        className={`block w-full rounded-inner border border-line-strong bg-transparent px-[15px] ${dense ? DENSE_FIELD : "py-3 text-[15px]"} text-ink motion-tint placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
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

/**
 * What is wrong with a field, under it. It appears still, on tint, and goes on
 * tint when the field is edited and the error stops being true; while it goes it
 * still reads what it said. Nothing moves: the page is pushed by the line, not
 * by the line's own motion. Renders nothing for no message.
 */
export function FieldError({
  children,
  className = "",
  ...rest
}: { children?: React.ReactNode } & Omit<React.HTMLAttributes<HTMLParagraphElement>, "children">) {
  const open = Boolean(children);
  const said = useRef(children);
  if (open) said.current = children;
  const { rendered, leaving } = useExitPhase(open, TINT_MS);
  if (!rendered) return null;
  return (
    <p
      role="alert"
      className={`mt-[7px] text-[13px] text-danger-text ${leaving ? "tint-out" : "tint-in"} ${className}`}
      {...rest}
    >
      {said.current}
    </p>
  );
}

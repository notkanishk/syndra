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
        className={`block w-full rounded-inner border border-line-strong bg-transparent px-[15px] py-3 text-[15px] text-ink transition-colors placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
        {...props}
      />
    );
  },
);

/** A field label — 12.5px/600, quiet, sitting directly above its control. */
export function FieldLabel({
  children,
  htmlFor,
  className = "",
}: {
  children: React.ReactNode;
  htmlFor?: string;
  className?: string;
}) {
  return (
    <label
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

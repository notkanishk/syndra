"use client";

import React from "react";

type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

/**
 * Pill-shaped native <select> wrapper. Custom multi-select is deferred —
 * native <select> with our pill styling covers every Stage 2 use-case.
 */
export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className = "", children, ...props },
  ref,
) {
  return (
    <select
      ref={ref}
      className={`block w-full rounded-full bg-surface-container px-4 py-2 text-sm text-on-surface shadow-[inset_0_1px_2px_rgba(0,0,0,0.4)] focus-visible:outline-2 focus-visible:outline-primary-container focus-visible:outline-offset-1 disabled:cursor-not-allowed disabled:opacity-50 ${className}`}
      {...props}
    >
      {children}
    </select>
  );
});

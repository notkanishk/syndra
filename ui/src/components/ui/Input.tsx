"use client";

import React from "react";

type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

/**
 * Pill-shaped text input with an inner shadow that reads as "recessed" inside
 * the surface, then lifts when focused. The focus ring uses
 * --primary-container so it harmonizes with primary buttons.
 */
export const Input = React.forwardRef<HTMLInputElement, InputProps>(function Input(
  { className = "", ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={`block w-full rounded-full bg-surface-container px-4 py-2 text-sm text-on-surface placeholder:text-on-surface-variant shadow-[inset_0_1px_2px_rgba(0,0,0,0.4)] focus-visible:outline-2 focus-visible:outline-primary-container focus-visible:outline-offset-1 disabled:cursor-not-allowed disabled:opacity-50 ${className}`}
      {...props}
    />
  );
});

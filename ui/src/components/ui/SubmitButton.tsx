"use client";

import React from "react";

interface SubmitButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isPending?: boolean;
  pendingLabel?: string;
  /** Override the visible label when not pending (children otherwise). */
  label?: string;
  /** "primary" (default) / "danger" / "success" — semantic color hint. */
  variant?: "primary" | "danger" | "success";
}

/**
 * Submit button with built-in busy state. While `isPending` is true, the
 * button is `disabled`, exposes `aria-busy`, and shows a spinner alongside
 * `pendingLabel`. Use across all create/update forms so users see a clear
 * "in flight" affordance instead of a static button + ambiguous network gap.
 */
export function SubmitButton({
  isPending = false,
  pendingLabel = "Saving…",
  label,
  variant = "primary",
  disabled,
  children,
  className = "",
  type = "submit",
  ...props
}: SubmitButtonProps) {
  const variantClasses =
    variant === "danger"
      ? "bg-error text-on-error hover:bg-error/90"
      : variant === "success"
        ? "bg-success text-on-success hover:bg-success-hover"
        : "bg-primary text-on-primary hover:bg-primary-container hover:text-on-primary-container";

  const isDisabled = disabled || isPending;

  return (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={isPending || undefined}
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-medium shadow-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50 ${variantClasses} ${className}`}
      {...props}
    >
      {isPending && (
        <span
          aria-hidden="true"
          className="h-4 w-4 animate-spin rounded-full border-2 border-current/40 border-t-current"
        />
      )}
      <span>{isPending ? pendingLabel : (label ?? children)}</span>
    </button>
  );
}

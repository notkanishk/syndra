"use client";

import React from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "success" | "warning";
type Size = "sm" | "md";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  isPending?: boolean;
}

/**
 * Primary, secondary, ghost, success, warning, and danger button variants
 * sharing focus-visible rings and busy-state spinners. Use this everywhere
 * an action button is rendered so visual hierarchy stays consistent across
 * pages.
 */
export function Button({
  variant = "primary",
  size = "md",
  isPending = false,
  disabled,
  className = "",
  type = "button",
  children,
  ...props
}: ButtonProps) {
  const sizeClasses = size === "sm" ? "px-2.5 py-1 text-xs" : "px-4 py-2 text-sm";
  const baseClasses =
    "inline-flex items-center justify-center gap-2 rounded-lg font-medium shadow-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50";

  const variants: Record<Variant, string> = {
    primary: "bg-primary hover:bg-primary-hover text-white",
    secondary: "border border-border bg-surface text-foreground hover:bg-surface-hover",
    ghost: "text-muted hover:text-foreground hover:bg-surface-hover",
    danger: "bg-red-500 hover:bg-red-600 text-white",
    success: "bg-emerald-500 hover:bg-emerald-600 text-white",
    warning: "bg-amber-500 hover:bg-amber-600 text-white",
  };

  const isDisabled = disabled || isPending;

  return (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={isPending || undefined}
      className={`${baseClasses} ${sizeClasses} ${variants[variant]} ${className}`}
      {...props}
    >
      {isPending && (
        <span
          aria-hidden="true"
          className="h-4 w-4 animate-spin rounded-full border-2 border-current/40 border-t-current"
        />
      )}
      <span>{children}</span>
    </button>
  );
}

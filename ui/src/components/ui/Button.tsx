"use client";

import React from "react";

type Variant =
  | "primary"
  | "secondary"
  | "ghost"
  | "outline"
  | "destructive"
  | "danger"
  | "success"
  | "warning"
  | "link";
type Size = "sm" | "md" | "lg";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  isPending?: boolean;
}

/**
 * Pill-shaped action button. Pill-default is intentional ("touchable, organic"
 * surface treatment per Obsidian Clarity). All existing variant names from
 * the prior Button are preserved (`danger`, `success`, `warning`) so callers
 * don't break; new variants `outline`, `destructive`, `link` extend the set.
 *
 * Primary uses a luminous indigo→violet gradient with a 1px white-10% inset
 * top stroke and an ambient shadow tinted to match — the "glass bead" effect
 * called out in the design spec.
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
  const sizeClasses =
    size === "sm"
      ? "px-3 py-1 text-xs"
      : size === "lg"
        ? "px-6 py-3 text-base"
        : "px-4 py-2 text-sm";

  const baseClasses =
    "inline-flex items-center justify-center gap-2 rounded-full font-medium transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container disabled:cursor-not-allowed disabled:opacity-50";

  // Note: the gradient + inset stroke + colored shadow combination is the
  // "glass bead" treatment. We keep the gradient inline (not a token) because
  // it composes two tokens and Tailwind v4 doesn't expose a clean way to mix
  // them via utilities. We use --primary and --secondary (the saturated
  // tokens, NOT the *-container variants) so on-primary white-on-saturated in
  // light theme and dark-on-luminous in dark theme both meet WCAG AA.
  const variants: Record<Variant, string> = {
    primary:
      "text-on-primary bg-[linear-gradient(135deg,var(--primary),var(--secondary))] shadow-[0_8px_24px_-8px_var(--primary),inset_0_1px_0_rgba(255,255,255,0.15)] hover:brightness-110",
    secondary:
      "bg-surface-container-high text-on-surface hover:bg-surface-container-highest shadow-sm",
    ghost: "text-on-surface-variant hover:text-on-surface hover:bg-surface-container-low",
    outline:
      "border border-outline-variant text-on-surface bg-transparent hover:bg-surface-container-low",
    destructive:
      "bg-error-container text-on-error-container hover:opacity-90 shadow-[0_8px_24px_-8px_var(--error-container)]",
    // `danger` is the historical name for destructive — alias for back-compat.
    danger:
      "bg-error-container text-on-error-container hover:opacity-90 shadow-[0_8px_24px_-8px_var(--error-container)]",
    success: "bg-[var(--success)] text-white hover:bg-[var(--success-hover)] shadow-sm",
    warning: "bg-[var(--warning)] text-white hover:bg-[var(--warning-hover)] shadow-sm",
    link: "text-primary-container hover:underline underline-offset-4 px-1 py-0 shadow-none",
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

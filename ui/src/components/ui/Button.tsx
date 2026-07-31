"use client";

import Link from "next/link";
import React from "react";

/**
 * Pill buttons. Semantic roles only, and one rule that is not negotiable:
 *
 *   `danger` is an OUTLINE. A solid destructive fill appears only on the
 *   confirming button inside a dialog (`dangerConfirm`) — a solid red button
 *   sitting in a table row is one stray click from an outage on the laser
 *   cutter.
 *
 * Disabled controls state their reason in visible copy, never only in a
 * title: hover does not exist on touch and does not survive a screenshot sent
 * to a colleague. Pass `reason` and it renders beneath the button.
 */

type Variant = "accent" | "accentSoft" | "outline" | "ghost" | "danger" | "dangerConfirm";
type Size = "sm" | "md";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  isPending?: boolean;
  /** Visible explanation rendered under a disabled control. */
  reason?: string;
}

const VARIANTS: Record<Variant, string> = {
  accent: "bg-accent text-accent-ink hover:brightness-105 active:brightness-95",
  accentSoft: "bg-accent-soft text-accent-text hover:brightness-110",
  outline: "border border-line-strong text-ink hover:bg-[var(--hover)]",
  ghost: "text-muted hover:text-ink hover:bg-[var(--hover)]",
  danger: "border border-danger-line text-danger-text hover:bg-danger-soft",
  dangerConfirm: "bg-danger text-danger-ink hover:brightness-105 active:brightness-95",
};

/** A blocked control keeps the semantic colour it would otherwise carry, at
 *  reduced alpha — it must still read as the destructive action it is. */
const DISABLED: Record<Variant, string> = {
  accent: "bg-accent/25 text-accent-ink/50",
  accentSoft: "bg-accent-soft/50 text-accent-text/40",
  outline: "border border-line text-faint",
  ghost: "text-faint",
  danger: "border border-danger/[.16] bg-danger/[.06] text-danger-text/40",
  dangerConfirm: "bg-danger/[.14] text-danger-text/40",
};

export function Button({
  variant = "outline",
  size = "md",
  isPending = false,
  reason,
  disabled,
  className = "",
  type = "button",
  children,
  ...props
}: ButtonProps) {
  const isDisabled = disabled || isPending;

  const button = (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={isPending || undefined}
      className={buttonClasses({ variant, size, disabled: isDisabled, className })}
      {...props}
    >
      {isPending && (
        <span
          aria-hidden
          className="h-3.5 w-3.5 animate-spin rounded-pill border-2 border-current/40 border-t-current"
        />
      )}
      {children}
    </button>
  );

  if (!reason) return button;
  return (
    <span className="inline-flex flex-col items-start gap-1.5">
      {button}
      <span className="max-w-[46ch] text-[12.5px] leading-[1.5] text-faint">{reason}</span>
    </span>
  );
}

/**
 * A navigation that looks like a button.
 *
 * Wrapping `<Button>` in a `<Link>` would nest a native `<button>` inside an
 * `<a>` — invalid HTML, two overlapping interactive elements, and unreliable
 * keyboard and screen-reader behaviour. Something that navigates is a link and
 * must be exactly one element; only the styling is shared.
 */
export function ButtonLink({
  href,
  variant = "outline",
  size = "md",
  className = "",
  children,
  ...props
}: Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> & {
  href: string;
  variant?: Variant;
  size?: Size;
}) {
  return (
    <Link href={href} className={buttonClasses({ variant, size, className })} {...props}>
      {children}
    </Link>
  );
}

/** The shared surface, so a link and a button are visually one control. */
function buttonClasses({
  variant,
  size,
  disabled = false,
  className = "",
}: {
  variant: Variant;
  size: Size;
  disabled?: boolean;
  className?: string;
}): string {
  const sizeClass = size === "sm" ? "px-3.5 py-1.5 text-[13px]" : "px-4 py-[7px] text-[13.5px]";
  return `inline-flex items-center justify-center gap-2 rounded-pill font-semibold transition-all duration-150 disabled:cursor-not-allowed ${sizeClass} ${
    disabled ? DISABLED[variant] : VARIANTS[variant]
  } ${className}`;
}

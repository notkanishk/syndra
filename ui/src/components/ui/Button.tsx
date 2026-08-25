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

/**
 * The pill metric, exported because several controls are this box and are not
 * this component: the view switch, the sound switch, the "how long" choice on a
 * request, `FilterPills`, `Tabs`. Each carries its own semantics — a radiogroup,
 * a switch, a pressed state — and forcing them through `Button` would mean props
 * only they use. What they must not each carry is their own idea of the numbers.
 *
 * They did. Across the product the same pill was px-3.5 or px-4, 13px or 13.5px
 * or 14.5px, and py-1.5 or py-[7px], so a row containing a filter, a tab and a
 * button showed three heights. Importing this is what stops that recurring;
 * `one-control-surface.test.ts` is what stops somebody typing it out again.
 *
 * The 44px floor holds through the tablet range on purpose — 720–1080 is a
 * tablet, which is still a thumb — and lifts above the desktop breakpoint,
 * where the control returns to its dense size.
 */
export const PILL: Record<Size, string> = {
  sm: "min-h-[44px] px-3.5 text-[13px] desktop:min-h-0 desktop:py-1.5",
  md: "min-h-[44px] px-4 text-[13.5px] desktop:min-h-0 desktop:py-[7px]",
};

/** Every label on this control is 13–13.5px, which is small text by WCAG at
 *  any weight — so a filled variant takes the dense accent, not the bright
 *  one. `--accent` under `--accent-ink` is 4.18:1 and fails; `--accent-dense`
 *  is 5.2:1 and passes. */
const VARIANTS: Record<Variant, string> = {
  accent: "bg-accent-dense text-accent-ink hover:brightness-110",
  accentSoft: "bg-accent-soft text-accent-text hover:brightness-110",
  outline: "border border-line-strong text-ink hover:bg-[var(--hover)]",
  ghost: "text-muted hover:text-ink hover:bg-[var(--hover)]",
  danger: "border border-danger-line text-danger-text hover:bg-danger-soft",
  dangerConfirm: "bg-danger text-danger-ink hover:brightness-110",
};

/** A blocked control keeps the semantic colour it would otherwise carry, at
 *  reduced alpha — it must still read as the destructive action it is. */
const DISABLED: Record<Variant, string> = {
  accent: "bg-accent-dense/25 text-accent-ink/50",
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
      {/* A dot on the product's one licensed loop, not a spinner. `breathe`
          already means "this is still happening" everywhere else in the
          system, and a second idiom for the same statement is one the
          operator has to learn twice. */}
      {isPending && (
        <span aria-hidden className="breathe h-2 w-2 flex-none rounded-pill bg-current" />
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
  // Every button in the product clears 44px until the desktop breakpoint, and
  // returns to its dense size above it. This is the single highest-leverage
  // line in the touch work: `sm` rendered about 30px tall and `md` about 32,
  // so without it every control in Syndra — approve, revoke, refresh, resume —
  // was a target a finger misses, and fixing them individually would be
  // several hundred edits that the next new button would not inherit.
  //
  // The floor stays up through the tablet range deliberately. 720–1080 is a
  // floor operator holding a tablet, which is still a thumb.
  const sizeClass = PILL[size];
  // `press`, and a 3% scale-down rather than a translate, so the button stays
  // under the finger. Destructive buttons behave identically — muscle memory
  // must never depend on what a button does.
  return `inline-flex items-center justify-center gap-2 rounded-pill font-semibold motion-press press-scale disabled:cursor-not-allowed ${sizeClass} ${
    disabled ? DISABLED[variant] : VARIANTS[variant]
  } ${className}`;
}

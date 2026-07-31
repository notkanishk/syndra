import React from "react";

/**
 * Count and status pills. Tone follows the semantic palette and nothing else:
 * neutral for a kind, accent for work, warn for a deadline, danger for
 * something that already went wrong.
 *
 * `hollow` renders the zero form — a count that is currently nothing still
 * occupies its seat rather than disappearing.
 */

type Tone = "neutral" | "accent" | "warn" | "danger";

const TONES: Record<Tone, string> = {
  neutral: "bg-tint-2 text-ink/[.82]",
  accent: "bg-accent text-accent-ink",
  warn: "bg-warn text-warn-ink",
  danger: "bg-danger text-danger-ink",
};

export function Badge({
  tone = "neutral",
  hollow = false,
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: Tone; hollow?: boolean }) {
  return (
    <span
      className={`inline-flex items-center rounded-pill px-2.5 py-0.5 text-[12px] font-semibold ${
        hollow ? "border border-line-strong text-label" : TONES[tone]
      } ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}

/** A neutral outline pill — bundle membership, an app served, a role group. */
export function Chip({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={`inline-flex items-center rounded-pill bg-tint-2 px-3.5 py-1.5 text-[14px] font-semibold ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}

/** A role key, grant id or claim name. Always mono, never the body face. */
export function Mono({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span className={`type-mono ${className}`} {...props}>
      {children}
    </span>
  );
}

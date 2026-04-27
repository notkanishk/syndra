import React from "react";

type PulseVariant = "success" | "warn" | "error" | "info";

interface PulseProps {
  variant?: PulseVariant;
  /** When true, the dot does not animate. Use for steady-state success indicators. */
  static?: boolean;
  className?: string;
  /** Optional aria-label for screen readers. */
  ariaLabel?: string;
}

const VARIANT_COLOR: Record<PulseVariant, string> = {
  success: "text-[var(--success)]",
  warn: "text-[var(--warning)]",
  error: "text-[var(--error)]",
  info: "text-[var(--info)]",
};

/**
 * Animated status indicator. Replaces the flat colored dot used to signal
 * live/in-flight/degraded state. Steady-success uses `static` prop to skip
 * the animation; the others animate by default per the Pulse semantics
 * defined in the operational-readiness spec.
 */
export function Pulse({
  variant = "info",
  static: isStatic = false,
  className = "",
  ariaLabel,
}: PulseProps) {
  return (
    <span
      role={ariaLabel ? "status" : undefined}
      aria-label={ariaLabel}
      className={`inline-block ${VARIANT_COLOR[variant]} ${isStatic ? "pulse-dot-static" : "pulse-dot"} ${className}`}
    />
  );
}

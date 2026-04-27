import React from "react";

type BadgeVariant =
  | "default"
  | "secondary"
  | "outline"
  | "destructive"
  | "success"
  | "warning"
  | "info";

interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: BadgeVariant;
  /** When true, overlays a small animated pulse dot at the trailing edge. */
  pulse?: boolean;
}

/**
 * Pill badge for status, tag, or count display. The `pulse` prop overlays a
 * small animated dot at the trailing edge to indicate live or in-flight
 * state — used on Operations page rows for in-flight intents and on the
 * watchlist for active escalations.
 */
export function Badge({
  className = "",
  variant = "default",
  pulse = false,
  children,
  ...props
}: BadgeProps) {
  const baseClasses =
    "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors";

  const variants: Record<BadgeVariant, string> = {
    default: "border-transparent bg-primary-container text-on-primary-container",
    secondary: "border-transparent bg-surface-container-high text-on-surface",
    outline: "border-outline-variant text-on-surface",
    destructive: "border-transparent bg-error-container text-on-error-container",
    success:
      "border-transparent bg-[color-mix(in_srgb,var(--success)_18%,transparent)] text-[var(--success)]",
    warning:
      "border-transparent bg-[color-mix(in_srgb,var(--warning)_18%,transparent)] text-[var(--warning)]",
    info: "border-transparent bg-[color-mix(in_srgb,var(--info)_18%,transparent)] text-[var(--info)]",
  };

  return (
    <div className={`${baseClasses} ${variants[variant]} ${className}`} {...props}>
      <span>{children}</span>
      {pulse && <span aria-hidden="true" className="pulse-dot" />}
    </div>
  );
}

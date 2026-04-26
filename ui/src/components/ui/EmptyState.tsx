import React from "react";
import Link from "next/link";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: { label: string; href?: string; onClick?: () => void };
  icon?: React.ReactNode;
  tone?: "neutral" | "destructive";
  className?: string;
}

/**
 * Explanatory empty/zero state used across the dashboard. Mirrors the
 * dashed-border treatment from the policies page so empty admin views read
 * intentionally — never blank cards or empty grids.
 */
export function EmptyState({
  title,
  description,
  action,
  icon,
  tone = "neutral",
  className = "",
}: EmptyStateProps) {
  const toneClasses =
    tone === "destructive"
      ? "border-red-500/30 bg-red-500/5"
      : "border-border bg-surface/40";

  return (
    <div
      role="status"
      aria-live="polite"
      className={`rounded-xl border border-dashed ${toneClasses} p-8 text-center ${className}`}
    >
      {icon && (
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary">
          {icon}
        </div>
      )}
      <p className="text-sm font-semibold text-foreground">{title}</p>
      {description && (
        <p className="mx-auto mt-2 max-w-md text-sm text-muted">{description}</p>
      )}
      {action && (
        <div className="mt-4">
          {action.href ? (
            <Link
              href={action.href}
              className="inline-flex rounded-lg bg-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white"
            >
              {action.label}
            </Link>
          ) : (
            <button
              type="button"
              onClick={action.onClick}
              className="inline-flex rounded-lg bg-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white"
            >
              {action.label}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

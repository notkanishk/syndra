import React from "react";
import Link from "next/link";

import { Eyebrow } from "@/components/ui/Eyebrow";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: { label: string; href?: string; onClick?: () => void };
  icon?: React.ReactNode;
  tone?: "neutral" | "destructive";
  /** Optional eyebrow label rendered above the title. */
  eyebrow?: string;
  className?: string;
}

/**
 * Glass-card empty/zero state. Replaces the dashed-border treatment with the
 * same translucent surface used by content cards so empty admin views feel
 * intentional, not unfinished. Eyebrow + headline + body + optional CTA.
 */
export function EmptyState({
  title,
  description,
  action,
  icon,
  tone = "neutral",
  eyebrow,
  className = "",
}: EmptyStateProps) {
  const toneClass =
    tone === "destructive"
      ? "text-[var(--error)]"
      : "text-on-surface-variant";

  return (
    <div
      role="status"
      aria-live="polite"
      className={`glass-card p-8 text-center ${className}`}
    >
      {icon && (
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-primary-container/30 text-primary-container">
          {icon}
        </div>
      )}
      {eyebrow && (
        <div className="mb-2">
          <Eyebrow tone="muted">{eyebrow}</Eyebrow>
        </div>
      )}
      <p className="text-base font-semibold text-on-surface">{title}</p>
      {description && (
        <p className={`mx-auto mt-2 max-w-md text-sm ${toneClass}`}>{description}</p>
      )}
      {action && (
        <div className="mt-5">
          {action.href ? (
            <Link
              href={action.href}
              className="inline-flex rounded-full bg-[linear-gradient(135deg,var(--primary),var(--secondary))] px-4 py-2 text-xs font-semibold uppercase tracking-[0.1em] text-on-primary shadow-[0_8px_24px_-8px_var(--primary),inset_0_1px_0_rgba(255,255,255,0.15)]"
            >
              {action.label}
            </Link>
          ) : (
            <button
              type="button"
              onClick={action.onClick}
              className="inline-flex rounded-full bg-[linear-gradient(135deg,var(--primary),var(--secondary))] px-4 py-2 text-xs font-semibold uppercase tracking-[0.1em] text-on-primary shadow-[0_8px_24px_-8px_var(--primary),inset_0_1px_0_rgba(255,255,255,0.15)]"
            >
              {action.label}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

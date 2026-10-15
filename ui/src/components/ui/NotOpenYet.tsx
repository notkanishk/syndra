import type { ReactNode } from "react";

import { StatusDot } from "@/components/ui/Badge";

/**
 * A section someone can't use yet (proposed/NotOpenYet.md).
 *
 * Dashed, a full-strength lead line saying what opens it and a link to that
 * step, and the contents drawn as they will be, at 0.45 and inert, so the
 * person sees what they are about to get. When `open`, it is only its
 * contents. Stale data is not this: it stays readable under its Notice.
 */
export function NotOpenYet({
  open,
  title,
  why,
  step,
  size = "card",
  children,
}: {
  open: boolean;
  title: string;
  why: string;
  /** The way on. Absent when there is none to take from here right now. */
  step?: { label: string; href: string };
  /** In a card (15px lead) or around a page's regions (19px display lead). */
  size?: "card" | "page";
  children: ReactNode;
}) {
  if (open) return <>{children}</>;
  const page = size === "page";
  return (
    <div
      className={`flex flex-col border border-dashed border-line-strong ${
        page ? "gap-[22px] rounded-card p-5" : "gap-3.5 rounded-inner px-4 pb-4 pt-3.5"
      }`}
    >
      <div className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <span
          className={`inline-flex items-center gap-2.5 font-semibold text-ink ${
            page ? "font-display text-[19px]" : "text-[15px]"
          }`}
        >
          <StatusDot tone="neutral" />
          {title}
        </span>
        <span className={`text-muted [text-wrap:pretty] ${page ? "text-[15px]" : "text-[14px]"}`}>{why}</span>
        {step && (
          <a
            href={step.href}
            className="ml-auto inline-flex min-h-[44px] items-center whitespace-nowrap text-[14px] font-semibold text-accent-text desktop:min-h-0"
          >
            {step.label}
          </a>
        )}
      </div>
      <div aria-disabled="true" inert className={`flex select-none flex-col opacity-45 ${page ? "gap-9" : "gap-3.5"}`}>
        {children}
      </div>
    </div>
  );
}

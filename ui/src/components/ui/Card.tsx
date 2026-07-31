import React from "react";

/**
 * The container surface: 20px radius, one hairline, no padding of its own so
 * a card can hold either a padded body or a run of full-bleed rows.
 *
 * `Card` + `CardHeader` + `CardRow` is the shape every list on the product
 * uses — a header carrying the title, a count badge and an optional right-side
 * note, then one row per item separated by a divider.
 */

export function Card({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`card overflow-hidden ${className}`} {...props}>
      {children}
    </div>
  );
}

export function CardHeader({
  title,
  count,
  tone = "accent",
  note,
  action,
  className = "",
}: {
  title: React.ReactNode;
  count?: number;
  tone?: "accent" | "warn" | "danger";
  /** Quiet right-aligned copy — a rule, a caveat, a timestamp. */
  note?: React.ReactNode;
  /** Right-aligned link or control. */
  action?: React.ReactNode;
  className?: string;
}) {
  const badgeTone =
    tone === "warn"
      ? "bg-warn text-warn-ink"
      : tone === "danger"
        ? "bg-danger text-danger-ink"
        : "bg-accent text-accent-ink";

  return (
    <div className={`flex flex-wrap items-center gap-[11px] px-5 py-4 ${className}`}>
      <span className="type-card-title">{title}</span>
      {count !== undefined && (
        <span className={`rounded-pill px-2.5 py-0.5 text-[12px] font-bold ${badgeTone}`}>
          {count}
        </span>
      )}
      <span className="flex-1" />
      {note && <span className="text-[13.5px] text-faint">{note}</span>}
      {action}
    </div>
  );
}

/** One item in a card. The first row omits its divider. */
export function CardRow({
  className = "",
  first = false,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { first?: boolean }) {
  return (
    <div
      className={`flex items-center gap-[18px] px-5 py-3.5 ${first ? "" : "row-divider"} ${className}`}
      {...props}
    >
      {children}
    </div>
  );
}

/** Column headings for a table-shaped card. */
export function CardColumns({
  className = "",
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className={`flex gap-[18px] px-5 py-[11px] text-[11.5px] font-semibold uppercase tracking-[0.09em] text-label ${className}`}
    >
      {children}
    </div>
  );
}

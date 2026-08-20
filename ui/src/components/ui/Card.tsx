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
        : "bg-accent-dense text-accent-ink";

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

/**
 * One item in a card. The first row omits its divider.
 *
 * On a phone a row that carries more than two facts discloses the rest rather
 * than truncating them: `disclosure` is the body, `expanded` says whether it is
 * open, and `onToggle` makes the WHOLE row the control. Never a chevron alone —
 * a 16px target inside a 60px row is a row that looks tappable and mostly
 * is not.
 *
 * Both props are optional and additive, so every existing caller is unchanged
 * and renders exactly as it did.
 */
export function CardRow({
  className = "",
  first = false,
  disclosure,
  expanded = false,
  onToggle,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  first?: boolean;
  disclosure?: React.ReactNode;
  expanded?: boolean;
  onToggle?: () => void;
}) {
  const line = (
    <div
      className={`flex min-h-[60px] items-center gap-[18px] px-5 py-3.5 tablet:min-h-0 ${
        first ? "" : "row-divider"
      } ${className}`}
      {...props}
    >
      {children}
    </div>
  );

  if (!disclosure || !onToggle) return line;

  return (
    <div className={first ? "" : "row-divider"}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        className="w-full text-left motion-press"
      >
        {/* `first` on the inner line so the divider is not drawn twice: it
            belongs to the wrapper now, which is what the next row sits under. */}
        <div className={`flex min-h-[60px] items-center gap-[18px] px-5 py-3.5 ${className}`}>
          {children}
        </div>
      </button>

      {/* Pushed, never overlapped. The rows below move down and the operator
          keeps their place in the list; an overlay would cover the row they
          were comparing this one against. */}
      {expanded && <div className="settle-in px-5 pb-4 pt-1">{disclosure}</div>}
    </div>
  );
}

/**
 * Column headings for a table-shaped card.
 *
 * Hidden below the tablet breakpoint, centrally rather than by every caller.
 * A column header is a promise that the cells beneath it line up, and on a
 * phone they do not — the row's fields stack, and each one carries its own
 * label inside the disclosure instead.
 */
export function CardColumns({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`hidden gap-[18px] px-5 py-[11px] text-[12.5px] font-semibold uppercase tracking-[0.09em] text-label tablet:flex ${className}`}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * The label/value pair a disclosed row is built from.
 *
 * A field that was a column on a wide screen becomes one of these, and the
 * label is what the column heading used to say — otherwise a disclosed value
 * is a string with no idea what it is.
 */
export function RowField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 py-1.5">
      <span className="type-label">{label}</span>
      <span className="text-[13.5px] text-ink/[.82]">{children}</span>
    </div>
  );
}

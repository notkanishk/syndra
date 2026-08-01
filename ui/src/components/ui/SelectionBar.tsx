"use client";

/**
 * The one selection bar.
 *
 * Docked at the bottom rather than inserted above the list: a bar that appears
 * in the flow pushes every row down the moment you tick the first checkbox,
 * which moves the row you were about to tick next out from under the cursor.
 * Structure does not move in response to data here either.
 *
 * It states the selection in words before offering any verb, because the count
 * is the thing an operator is about to be wrong about — "47" read at a glance
 * is indistinguishable from "4" or "470", and the difference is dozens of
 * people's access. `composition` is the second half of that: what the selection
 * is made of, not just how much of it there is.
 */

interface SelectionBarProps {
  count: number;
  /** Singular/plural noun for what is selected: "person"/"people", "item"/"items". */
  noun: [string, string];
  /** Where the selection came from — "in Laser Lab", "matching 'ada'". */
  scope?: string;
  /** What it is made of — "8 safety-gated · 3 people". */
  composition?: string;
  /** True when the selection came from select-all rather than individual clicks. */
  wholeScope?: boolean;
  /** Rows currently rendered; the escape hatch offers to narrow to these. */
  visibleCount?: number;
  onSelectVisibleOnly?: () => void;
  onClear: () => void;
  /** The verbs. Rendered right-aligned, destructive ones marked. */
  children: React.ReactNode;
}

export function SelectionBar({
  count,
  noun,
  scope,
  composition,
  wholeScope = false,
  visibleCount,
  onSelectVisibleOnly,
  onClear,
  children,
}: SelectionBarProps) {
  if (count === 0) return null;

  const things = `${count} ${count === 1 ? noun[0] : noun[1]}`;
  const canNarrow =
    wholeScope && visibleCount !== undefined && onSelectVisibleOnly && count > visibleCount;

  return (
    <div
      role="region"
      aria-label="Selection"
      className="sticky bottom-4 z-20 mt-2 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[18px] border border-line-strong bg-surface-2 px-5 py-3.5 shadow-dialog"
    >
      <span className="text-[14.5px]">
        <strong className="font-semibold">
          {wholeScope ? `All ${things}` : things} selected
        </strong>
        {scope ? <span className="text-muted"> {scope}</span> : null}
        {composition ? <span className="text-faint"> · {composition}</span> : null}
        {canNarrow ? (
          <>
            {" — "}
            <button
              type="button"
              onClick={onSelectVisibleOnly}
              className="font-semibold text-accent-text underline-offset-2 hover:underline"
            >
              select only the {visibleCount} shown
            </button>
          </>
        ) : null}
      </span>

      <span className="flex-1" />

      <div className="flex flex-wrap items-center gap-2">
        {children}
        <button
          type="button"
          onClick={onClear}
          className="rounded-pill px-3 py-1.5 text-[13px] font-semibold text-muted transition-colors hover:text-ink"
        >
          Clear
        </button>
      </div>
    </div>
  );
}

/** A verb in the selection bar. `tone="danger"` for anything that takes access away. */
export function SelectionAction({
  onClick,
  tone = "neutral",
  disabled = false,
  children,
}: {
  onClick: () => void;
  tone?: "neutral" | "danger";
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`rounded-pill border px-3.5 py-1.5 text-[13px] font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
        tone === "danger"
          ? "border-danger-line text-danger-text hover:bg-danger-soft"
          : "border-line-strong hover:bg-[var(--hover)]"
      }`}
    >
      {children}
    </button>
  );
}

/** The header checkbox. Same control on every list that has one. */
export function SelectAllCheckbox({
  label,
  ...props
}: {
  label: string;
  checked: boolean;
  ref: (node: HTMLInputElement | null) => void;
  onChange: () => void;
}) {
  return (
    <input
      type="checkbox"
      aria-label={label}
      className="h-4 w-4 accent-[var(--accent)]"
      {...props}
    />
  );
}

/** A row checkbox. Spread `selection.checkboxProps(id)` onto it. */
export function RowCheckbox({
  label,
  ...props
}: {
  label: string;
} & Record<string, unknown>) {
  return (
    <input
      type="checkbox"
      aria-label={label}
      className="h-4 w-4 accent-[var(--accent)]"
      {...props}
    />
  );
}

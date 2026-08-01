"use client";

import type { BulkOp } from "@/lib/queries/useBulkGrants";

/**
 * The docked selection bar.
 *
 * It states the selection in words before offering a single verb, because the
 * number is the thing an operator is about to be wrong about: "47" read at a
 * glance is indistinguishable from "4" or "470", and the difference is dozens
 * of people's access. The sentence is the safety feature; the buttons are just
 * buttons.
 */

const ACTIONS: Array<{ op: BulkOp; label: string; danger?: boolean }> = [
  { op: "assign_role", label: "Grant role" },
  { op: "assign_bundle", label: "Add to bundle" },
  { op: "extend", label: "Extend expiring" },
  { op: "remove_bundle", label: "Remove bundle", danger: true },
  { op: "remove_role", label: "Remove role", danger: true },
];

interface BulkBarProps {
  count: number;
  /** The sentence describing what the current filter narrowed to, if anything. */
  scope: string;
  /** True when the selection came from "select all matching", not clicks. */
  wholeFilter: boolean;
  /** Rows currently rendered — the escape hatch offers to narrow to these. */
  visibleCount: number;
  onSelectVisibleOnly: () => void;
  onClear: () => void;
  onAct: (op: BulkOp) => void;
}

export function BulkBar({
  count,
  scope,
  wholeFilter,
  visibleCount,
  onSelectVisibleOnly,
  onClear,
  onAct,
}: BulkBarProps) {
  if (count === 0) return null;

  const people = `${count} ${count === 1 ? "person" : "people"}`;

  return (
    <div
      role="region"
      aria-label="Bulk actions"
      className="sticky bottom-4 z-20 mt-2 flex flex-wrap items-center gap-3 rounded-[18px] border border-line-strong bg-surface-2 px-5 py-3.5 shadow-dialog"
    >
      <span className="text-[14.5px]">
        <strong className="font-semibold">
          {wholeFilter ? `All ${people}` : people} selected
        </strong>
        {scope ? <span className="text-muted"> {scope}</span> : null}
        {wholeFilter && count > visibleCount ? (
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
        {ACTIONS.map((action) => (
          <button
            key={action.op}
            type="button"
            onClick={() => onAct(action.op)}
            className={`rounded-pill border px-3.5 py-1.5 text-[13px] font-semibold transition-colors ${
              action.danger
                ? "border-danger-line text-danger-text hover:bg-danger-soft"
                : "border-line-strong hover:bg-[var(--hover)]"
            }`}
          >
            {action.label}
          </button>
        ))}
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

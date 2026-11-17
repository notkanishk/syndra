"use client";

/** One row of a system's settings card (A-03 §1): the label, the value, its way on in the right seat. */
export function SettingRow({
  label,
  action,
  children,
}: {
  label: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="row-divider flex flex-wrap items-center gap-x-(--row-gap) gap-y-1 px-(--card-x) py-(--row-y) first:border-t-0">
      <span className="type-label w-(--seat-label) flex-none text-label">{label}</span>
      <span className="min-w-0 flex-[1_1_240px]">{children}</span>
      {/* The action seat (D-1 §4), the table's width: wraps right-aligned when the row is narrow, never under the label. */}
      <span data-action-seat className="row-seat">
        {action}
      </span>
    </div>
  );
}

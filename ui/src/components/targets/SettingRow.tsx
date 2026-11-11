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
    <div className="row-divider flex flex-wrap items-center gap-x-5 gap-y-1.5 px-5 py-3 first:border-t-0">
      <span className="type-label w-[170px] flex-none text-label">{label}</span>
      <span className="min-w-0 flex-[1_1_240px]">{children}</span>
      {/* Wraps right-aligned when the row is narrow, never under the label. */}
      <span className="ml-auto flex w-[200px] flex-none justify-end">{action}</span>
    </div>
  );
}

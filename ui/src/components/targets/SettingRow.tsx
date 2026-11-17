"use client";

import { CardRow } from "@/components/ui/Card";

/**
 * One row of a system's settings card (A-03 §1): the label, the value, its way
 * on in the right seat. A CardRow, so it is built the one way every row is
 * (D-1 §4): the label in the label seat, the action in the default seat, which
 * stays in its place when a row has no way on.
 */
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
    <CardRow className="first:border-t-0" seat="default" action={action}>
      <span className="type-label w-(--seat-label) flex-none text-label">{label}</span>
      <span className="min-w-0 flex-[1_1_240px]">{children}</span>
    </CardRow>
  );
}

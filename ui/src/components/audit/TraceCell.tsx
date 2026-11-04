"use client";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { traceFor } from "@/lib/audit-vocabulary";
import type { AuditEntry } from "@/lib/queries/useAudit";

/**
 * Where a row's edit led, in the two places it appears: the audit log and a
 * person's Activity tab. One link, only for a row the backend recorded a
 * cascade for (see `traceFor`); every other row says nothing, and no raw
 * handle is ever shown (A-17 G10, G17).
 */
export function TraceCell({ entry, className }: { entry: AuditEntry; className?: string }) {
  const trace = traceFor(entry);
  if (trace.kind !== "cascade") return null;
  return (
    <span className={className}>
      <ArrowLink href={trace.href}>What it set off</ArrowLink>
    </span>
  );
}

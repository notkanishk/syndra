"use client";

import Link from "next/link";

import { Mono } from "@/components/ui/Badge";
import { traceFor } from "@/lib/audit-vocabulary";
import type { AuditEntry } from "@/lib/queries/useAudit";

/**
 * The Trace column, in the two places it appears — the audit log and a person's
 * Activity tab. Shared rather than duplicated for the same reason the action
 * vocabulary is: an operator comparing the two screens must not find the same
 * row tracing to two different things.
 *
 * Three states, and the distinction between them is the whole point (see
 * `traceFor`): a real cascade links, a pre-lineage row names its object without
 * pretending to be one, and everything else is a dash.
 */
export function TraceCell({ entry, className }: { entry: AuditEntry; className?: string }) {
  const trace = traceFor(entry);

  return (
    <span className={className}>
      {trace.kind === "cascade" ? (
        <Link
          href={trace.href}
          aria-label="See what this change set off, in Change history"
          className="text-[13px] font-semibold text-accent-text"
        >
          <Mono>{trace.label}</Mono>
        </Link>
      ) : trace.kind === "object" ? (
        // The identity is shown, not hovered. It lived in a `title` — which
        // touch cannot open, screenshots do not carry, and this product
        // forbids everywhere else — and it is the only thing that says WHAT
        // this row traced to when there is no cascade to link.
        <span className="flex flex-col gap-0.5">
          <Mono className="text-[13px] text-faint">{trace.label}</Mono>
          {trace.title && (
            <span className="text-[12.5px] leading-[1.4] text-faint">{trace.title}</span>
          )}
        </span>
      ) : (
        <span className="text-[13px] text-faint">—</span>
      )}
    </span>
  );
}

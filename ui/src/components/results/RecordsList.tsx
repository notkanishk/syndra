"use client";

import { dueDate, emptyRecords, sinceDate, type Reader } from "@/components/results/copy";
import { observed, ReadFreshness, useReadAgainPress, useReadAgainSince } from "@/components/ui/ReadFreshness";
import { StandingLine } from "@/components/ui/StandingLine";
import { formatShortDate } from "@/lib/format";
import type { RecordsResult } from "@/lib/queries/useMyStorage";
import { sortByName } from "@/lib/sort";

/**
 * What a person holds, one row each: its label, its category, the two dates it
 * carries, and the add-on's own standing for it with that standing's next step.
 * Whether a record is late is the add-on's word (its standing), never worked
 * out here from a date.
 */
export function RecordsList({ result, name, reader }: { result: RecordsResult; name: string; reader: Reader }) {
  const again = useReadAgainPress();
  const since = useReadAgainSince();
  return (
    <div className="flex flex-col gap-3">
      {result.records.length === 0 ? (
        <p className="m-0 text-[14px] text-muted">{emptyRecords(name, reader)}</p>
      ) : (
        <ul className="m-0 flex list-none flex-col p-0">
          {sortByName(result.records, (r) => r.label).map((r) => (
            <li key={r.id} className="row-divider flex flex-wrap items-start gap-x-4 gap-y-2 py-3 first:border-t-0 first:pt-0">
              <span className="flex min-w-[200px] flex-1 flex-col gap-0.5">
                <span className="text-[14.5px] font-semibold [overflow-wrap:anywhere]">{r.label}</span>
                <span className="text-[13px] text-faint">
                  {[r.category, r.since && sinceDate(formatShortDate(r.since)), r.due && dueDate(formatShortDate(r.due))]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
              {r.standing && <StandingLine standing={r.standing} system={name} audience={reader.audience} />}
            </li>
          ))}
        </ul>
      )}
      <ReadFreshness state={observed(result, since)} subject={name} reader={reader} onRefresh={again} />
    </div>
  );
}

"use client";

import { dueDate, emptyRecords, RECORD_HEADS, RECORD_SORT_BY, sinceDate, type Reader } from "@/components/results/copy";
import { CardColumns } from "@/components/ui/Card";
import { observed, ReadFreshness, useReadAgainPress, useReadAgainSince } from "@/components/ui/ReadFreshness";
import { orderBy, SortHead, useSort } from "@/components/ui/SortHead";
import { StandingLine } from "@/components/ui/StandingLine";
import { formatShortDate } from "@/lib/format";
import type { RecordsResult } from "@/lib/queries/useMyStorage";
import { sortByName } from "@/lib/sort";

type Held = RecordsResult["records"][number];
type RecordSort = "record" | "since" | "due";

/**
 * A date as an instant, never worked out as "late" here. A record with no date
 * for the column in force sits last whichever way the column runs: `orderBy`
 * multiplies by the direction, so the stand-in flips with it.
 */
const when = (iso: string | undefined) => (iso ? Date.parse(iso) : null);

/**
 * What a person holds, one row each: its label, its category, the two dates it
 * carries, and the add-on's own standing for it with that standing's next step.
 * Whether a record is late is the add-on's word (its standing), never worked
 * out here from a date.
 *
 * The order is the most logical one (owner, 13 Nov 2026), not alphabetical:
 * soonest due first, so an earlier date (what is overdue) comes before what is
 * due soon, then records with no due date, then by name. A heading re-sorts by
 * its column; ties keep name order.
 */
export function RecordsList({ result, name, reader }: { result: RecordsResult; name: string; reader: Reader }) {
  const again = useReadAgainPress();
  const since = useReadAgainSince();
  const [sort, setSort, view] = useSort<RecordSort>("due");
  const by: Record<RecordSort, (r: Held) => string | number | null> = {
    record: (r) => r.label,
    since: (r) => when(r.since),
    due: (r) => when(r.due),
  };
  // Name first, then the stable sort: equal dates fall back to the name order.
  const rows = orderBy(sortByName(result.records, (r) => r.label), view.sort, by);
  return (
    <div className="flex flex-col gap-3">
      {result.records.length === 0 ? (
        <p className="m-0 text-[14px] text-muted">{emptyRecords(name, reader)}</p>
      ) : (
        <div>
          <CardColumns className="!px-0">
            <span className="min-w-[200px] flex-1">
              <SortHead label={RECORD_HEADS.record} by={RECORD_SORT_BY.record} name="record" sort={sort} onSort={setSort} />
            </span>
            <span className="w-[110px] shrink-0">
              <SortHead label={RECORD_HEADS.since} by={RECORD_SORT_BY.since} name="since" sort={sort} onSort={setSort} />
            </span>
            <span className="w-[110px] shrink-0">
              <SortHead label={RECORD_HEADS.due} by={RECORD_SORT_BY.due} name="due" sort={sort} onSort={setSort} />
            </span>
            <span aria-hidden className="w-[240px] shrink-0" />
          </CardColumns>
          <ul className={`m-0 flex list-none flex-col p-0 ${view.className}`}>
            {rows.map((r) => (
              <li key={r.id} className="row-divider flex flex-wrap items-start gap-x-4 gap-y-2 py-3 first:border-t-0 first:pt-0 tablet:flex-nowrap tablet:gap-x-[18px]">
                <span className="flex min-w-[200px] flex-1 flex-col gap-0.5">
                  <span className="text-[14.5px] font-semibold [overflow-wrap:anywhere]">{r.label}</span>
                  {r.category && <span className="text-[13px] text-faint">{r.category}</span>}
                </span>
                <span className="text-[13px] text-faint tablet:w-[110px] tablet:shrink-0">{r.since && sinceDate(formatShortDate(r.since))}</span>
                <span className="text-[13px] text-faint tablet:w-[110px] tablet:shrink-0">{r.due && dueDate(formatShortDate(r.due))}</span>
                <span className="min-w-0 tablet:w-[240px] tablet:shrink-0">
                  {r.standing && <StandingLine standing={r.standing} system={name} audience={reader.audience} />}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
      <ReadFreshness state={observed(result, since)} subject={name} reader={reader} onRefresh={again} />
    </div>
  );
}

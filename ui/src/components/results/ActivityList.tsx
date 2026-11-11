"use client";

import type { ReactNode } from "react";

import { emptyEvents, uncoveredNote } from "@/components/results/copy";
import { Badge } from "@/components/ui/Badge";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { formatClock, formatList } from "@/lib/format";
import type { ActivityResult } from "@/lib/queries/useMyStorage";
import type { TargetActivityEvent } from "@/lib/queries/useTargetActivity";

/**
 * What a system's own log holds for one person: the target's account of
 * itself, not Syndra's record. `resource` is a column of its own (a share, a
 * door, a printer: the same vocabulary as the places that were not recording).
 */
export function ActivityRows({ events }: { events: TargetActivityEvent[] }) {
  return (
    <>
      {events.map((event, at) => (
        <div key={`${event.at}-${at}`} className="row-divider flex flex-wrap items-baseline gap-x-4 gap-y-1 px-5 py-3">
          <span className="w-[64px] shrink-0 text-[12.5px] text-faint">{formatClock(event.at)}</span>
          <span className="min-w-[160px] flex-1 text-[14px]">
            <span className="font-semibold">{event.event}</span>
            {/* The reason, in the target's own vocabulary rather than a
                translation of it. `NT_STATUS_NO_SUCH_USER` is the string an
                operator searches for, and inventing friendlier wording would
                make it unfindable. */}
            {event.detail ? <span className="ml-2 font-mono text-[12.5px] text-faint">{event.detail}</span> : null}
          </span>
          <span className="w-[120px] shrink-0 text-[13px] text-muted [overflow-wrap:anywhere]" data-resource>
            {event.resource ?? ""}
          </span>
          {/* Where from. A week of refusals is a week of refusals from nowhere
              without it: the address is what tells one stale saved password
              from somebody working through the building. */}
          <span className="w-[120px] shrink-0 font-mono text-[12.5px] text-faint">{event.address ?? ""}</span>
          {/* Refusals are the rows worth finding. */}
          {!event.success && <Badge tone="warnSoft">Refused</Badge>}
        </div>
      ))}
    </>
  );
}

/**
 * The whole read: the rows, the places that did not record this person, and
 * how old it is. `coverage` lets a kind say the places in its own words; the
 * default says them in none.
 */
export function ActivityList({ result, name, coverage }: { result: ActivityResult; name: string; coverage?: ReactNode }) {
  const uncovered = result.uncovered ?? [];
  return (
    <div className="-mx-5 flex flex-col gap-3 [&>:first-child]:border-t-0">
      {result.events.length === 0 ? (
        <p className="m-0 px-5 text-[14px] text-muted">{emptyEvents(name)}</p>
      ) : (
        <ActivityRows events={result.events} />
      )}
      {uncovered.length > 0 && (
        <p className="m-0 px-5 text-[13px] text-faint">{coverage ?? uncoveredNote(formatList(uncovered), name)}</p>
      )}
      <ReadFreshness
        className="px-5"
        state={{ readAt: result.taken_at, current: result.current, truncated: result.truncated }}
        subject={name}
      />
    </div>
  );
}

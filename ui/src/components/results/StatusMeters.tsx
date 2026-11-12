"use client";

import { NO_LIMIT, emptyMeters, notReadableNext, ofLimit, usageUnread, usageUnreadBody, type Reader } from "@/components/results/copy";
import { Meter } from "@/components/ui/Meter";
import { Notice } from "@/components/ui/Notice";
import { observed, ReadFreshness, useReadAgainPress, useReadAgainSince } from "@/components/ui/ReadFreshness";
import { formatLimit } from "@/lib/fields";
import { sortByName } from "@/lib/sort";
import type { StatusResult } from "@/lib/queries/useMyStorage";

/** A number in its unit's words; a unit the core has no words for is written after the number. */
function amount(n: number, unit: string): string {
  const said = formatLimit(n, unit);
  return said === String(n) && unit !== "count" ? `${said} ${unit}` : said;
}

/**
 * What is used against what is allowed, one meter each. A meter with no limit
 * says so rather than drawing a bar against nothing. When the add-on says some
 * usage could not be read, the meters that were read stay, and the rest is
 * said not to be known: never empty meters standing in for "nothing used".
 */
export function StatusMeters({ result, name, reader }: { result: StatusResult; name: string; reader: Reader }) {
  const again = useReadAgainPress();
  const since = useReadAgainSince();
  const unread = result.quota_read === false;
  const meters = sortByName(result.meters ?? [], (m) => m.label, (m) => m.label);
  return (
    <div className="flex flex-col gap-3">
      {meters.length === 0 && !unread ? (
        <p className="m-0 text-[14px] text-muted">{emptyMeters(name)}</p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {meters.map((m) => (
            <li key={m.label} className="flex flex-col gap-1.5">
              <span className="flex flex-wrap items-baseline justify-between gap-x-3">
                <span className="text-[14px] font-semibold [overflow-wrap:anywhere]">{m.label}</span>
                <span className="text-[13px] text-muted">
                  {amount(m.used, m.unit)} · {m.limit === undefined ? NO_LIMIT : ofLimit(amount(m.limit, m.unit))}
                </span>
              </span>
              {m.limit !== undefined && m.limit > 0 && (
                <Meter value={m.used} limit={m.limit} label={m.label} valueText={`${amount(m.used, m.unit)} ${ofLimit(amount(m.limit, m.unit))}`} />
              )}
            </li>
          ))}
        </ul>
      )}
      {unread && (
        <UsageUnread name={name} some={meters.length > 0} reader={reader} />
      )}
      <ReadFreshness state={observed(result, since)} subject={name} reader={reader} onRefresh={again} />
    </div>
  );
}

/** Usage the add-on could not read, whole or in part: said, never drawn as zero. The storage cards use it too. */
export function UsageUnread({ name, some, reader }: { name: string; some: boolean; reader: Reader }) {
  return (
    <Notice tone="warn" title={usageUnread(name, some)} next={notReadableNext(reader)}>
      {usageUnreadBody(some)}
    </Notice>
  );
}

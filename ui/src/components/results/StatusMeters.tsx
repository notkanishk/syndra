"use client";

import { NO_LIMIT, emptyMeters, notReadableNext, ofLimit, usageUnread, usageUnreadBody, type Reader } from "@/components/results/copy";
import { Notice } from "@/components/ui/Notice";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
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
                <span
                  role="meter"
                  aria-label={m.label}
                  aria-valuemin={0}
                  aria-valuemax={m.limit}
                  aria-valuenow={Math.min(m.used, m.limit)}
                  className="block h-1.5 overflow-hidden rounded-pill bg-tint-2"
                >
                  <span className="block h-full rounded-pill bg-accent" style={{ width: `${Math.min(100, (m.used / m.limit) * 100)}%` }} />
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {unread && (
        <UsageUnread name={name} some={meters.length > 0} reader={reader} />
      )}
      <ReadFreshness state={{ readAt: result.taken_at, current: result.current }} subject={name} />
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

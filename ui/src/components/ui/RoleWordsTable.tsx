"use client";

import { Checkbox } from "@/components/ui/Choice";
import { CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { Ticks } from "@/components/ui/TokenBlock";
import { WaitsMark } from "@/components/ui/WaitsMark";

export interface RoleWordRow {
  key: string;
  name: string;
  waits?: boolean;
  /** The word the app sees; "" means the key, shown as the placeholder. */
  word: string;
  included: boolean;
  /** The word or inclusion differs from what's saved: the field keeps its border. */
  changed?: boolean;
  /** Faint lines under the word: "Was `netbird-staff`.", "Also Admins' word.", "New. …". */
  notes?: string[];
}

/**
 * Role · "<App> calls it" · Included (A-14 §3). Every project role, A to Z by
 * name; a short list, so no search. A role left out keeps its word readable
 * at muted strength and can't be edited until it is ticked again. A rename or
 * a removal says what it was on its own row, before the before-line and the
 * save summary say it again.
 *
 * The word fields are quiet at rest (A-15 §3): they read as mono text, show
 * their line on hover and their ring on focus, and one click edits. A field
 * whose row has changed keeps its border while it differs from what's saved.
 */
export function RoleWordsTable({
  app,
  rows,
  readOnly = false,
  onWord,
  onIncluded,
}: {
  app: string;
  rows: RoleWordRow[];
  readOnly?: boolean;
  onWord: (key: string, word: string) => void;
  onIncluded: (key: string, included: boolean) => void;
}) {
  const [sort, setSort, view] = useSort<"role">("role");
  const ordered = orderBy(rows, view.sort, { role: (r) => r.name });
  return (
    <div>
      <CardColumns>
        <span className="w-[170px] flex-none">
          <SortHead label="Role" name="role" sort={sort} onSort={setSort} />
        </span>
        <span className="min-w-0 flex-1">{app} calls it</span>
        <span className="w-[90px] flex-none text-right">Included</span>
      </CardColumns>
      <div className={view.className}>
      {ordered.map((r) => (
        <div key={r.key} className="row-divider flex flex-wrap items-start gap-x-3.5 gap-y-1.5 px-5 py-2">
          <span className="flex w-[170px] flex-none flex-col gap-px pt-0.5">
            <span className="inline-flex items-center gap-[7px] text-[14.5px] font-semibold">
              {r.name}
              {r.waits && <WaitsMark />}
            </span>
            <span className="type-mono text-[12.5px] text-faint">{r.key}</span>
          </span>
          <span className="flex min-w-0 flex-[1_1_170px] flex-col gap-1">
            <Input
              value={r.word}
              placeholder={r.key}
              spellCheck={false}
              readOnly={readOnly}
              disabled={!r.included}
              aria-label={`${app} calls ${r.name}`}
              onChange={(e) => onWord(r.key, e.target.value)}
              className={`w-full font-mono text-[13.5px] ${!r.included ? "border-transparent! text-muted" : r.changed ? "" : "border-transparent! hover:border-line! focus:border-accent-line!"}`}
            />
            {r.notes?.map((n) => (
              <span key={n} className="text-[12.5px] leading-[1.45] text-faint [text-wrap:pretty]">
                <Ticks text={n} />
              </span>
            ))}
          </span>
          <span className="flex w-[90px] flex-none justify-end pt-[11px]">
            <Checkbox
              checked={r.included}
              disabled={readOnly}
              aria-label={`${app} sees ${r.name}`}
              onChange={(e) => onIncluded(r.key, e.target.checked)}
            />
          </span>
        </div>
      ))}
      </div>
    </div>
  );
}

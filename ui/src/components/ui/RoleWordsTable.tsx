"use client";

import { Checkbox } from "@/components/ui/Choice";
import { CardColumns, Cells, type Columns } from "@/components/ui/Card";
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
 *
 * `dense` (A-16 §2) is the claims editor's: one line a role, about 44px, with
 * the name, its mark and its key side by side in a 190px seat, the word field
 * at the sm control's height and a 70px Included seat, indented to the key of
 * the claim line that opened it. A changed row grows only by its notes. On a
 * phone the name and key sit above the field.
 */
/** Role, what the app calls it, whether it is included; the dense table is narrower in its seats. */
const COLS: Columns = [
  "min-w-[140px] flex-[1_1_0%]",
  "min-w-[120px] flex-[2_1_0%]",
  "w-[90px] flex-none text-right",
];
const DENSE_COLS: Columns = [
  "w-full tablet:w-auto tablet:min-w-[140px] tablet:flex-[1_1_0%]",
  "min-w-[120px] flex-[2_1_0%]",
  "w-[70px] flex-none text-right",
];

export function RoleWordsTable({
  app,
  rows,
  readOnly = false,
  dense = false,
  onWord,
  onIncluded,
}: {
  app: string;
  rows: RoleWordRow[];
  readOnly?: boolean;
  dense?: boolean;
  onWord: (key: string, word: string) => void;
  onIncluded: (key: string, included: boolean) => void;
}) {
  const [sort, setSort, view] = useSort<"role">("role");
  const ordered = orderBy(rows, view.sort, { role: (r) => r.name });
  const cols = dense ? DENSE_COLS : COLS;
  return (
    <div>
      <div className={dense ? "tablet:pl-[47px]" : ""}>
        <CardColumns cols={cols}>
          <SortHead label="Role" name="role" sort={sort} onSort={setSort} />
          <span>{app} calls it</span>
          <span>Included</span>
        </CardColumns>
      </div>
      <div className={view.className}>
        {ordered.map((r) => (
          <div
            key={r.key}
            className={`row-divider row-cols flex flex-wrap ${
              dense
                ? "items-center gap-y-0.5 py-[5px] pl-5 pr-[22px] tablet:pl-[67px]"
                : "items-start gap-y-1.5 px-5 py-2"
            }`}
          >
            <Cells cols={cols}>
            {dense ? (
              <span className="inline-flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span className={`text-body font-semibold ${r.included ? "text-ink" : "text-muted"}`}>{r.name}</span>
                {r.waits && <WaitsMark />}
                <span className="type-mono text-fine text-faint">{r.key}</span>
              </span>
            ) : (
              <span className="flex flex-col gap-px pt-0.5">
                <span className="inline-flex items-center gap-[7px] text-body font-semibold">
                  {r.name}
                  {r.waits && <WaitsMark />}
                </span>
                <span className="type-mono text-fine text-faint">{r.key}</span>
              </span>
            )}
            <span className={`flex flex-col ${dense ? "gap-0.5" : "gap-1"}`}>
              <Input
                dense={dense}
                value={r.word}
                placeholder={r.key}
                spellCheck={false}
                readOnly={readOnly}
                disabled={!r.included}
                aria-label={`${app} calls ${r.name}`}
                onChange={(e) => onWord(r.key, e.target.value)}
                className={`w-full font-mono ${dense ? "" : "text-small"} ${!r.included ? "border-transparent! text-muted" : r.changed ? "" : "border-transparent! hover:border-line! focus:border-accent-line!"}`}
              />
              {r.notes?.map((n) => (
                <span key={n} className="text-fine leading-[1.45] text-faint [text-wrap:pretty]">
                  <Ticks text={n} />
                </span>
              ))}
            </span>
            <span className={`flex justify-end ${dense ? "" : "pt-[11px]"}`}>
              <Checkbox
                checked={r.included}
                disabled={readOnly}
                aria-label={`${app} sees ${r.name}`}
                onChange={(e) => onIncluded(r.key, e.target.checked)}
              />
            </span>
            </Cells>
          </div>
        ))}
      </div>
    </div>
  );
}

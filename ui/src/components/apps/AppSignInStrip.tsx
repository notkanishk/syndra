"use client";

import type { AppSignIns } from "@/lib/queries/useSignIns";

const DAYS = 30;
const TALL = 20;

/** The last 30 calendar days, oldest first, as YYYY-MM-DD. */
export function lastDays(now: Date = new Date(), n = DAYS): string[] {
  const out: string[] = [];
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i);
    out.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`);
  }
  return out;
}

/**
 * An app's sign-ins per day, last 30 days, naming nobody (A-10 §7; owner,
 * 3 Nov: "1a with statistics"), as the first row of the app page's tools card
 * (A-15 §6). One bar a day; a day with none is a hairline, never a gap.
 */
export function AppSignInStrip({ usage }: { usage: AppSignIns }) {
  const byDay = new Map((usage.days ?? []).map((d) => [d.day, d.count]));
  const counts = lastDays().map((day) => byDay.get(day) ?? 0);
  const max = Math.max(1, ...counts);
  const total = counts.reduce((a, b) => a + b, 0);
  const people = usage.people_30d ?? 0;
  return (
    <div className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-[22px] py-4">
      <span className="w-[170px] flex-none text-[14px] font-semibold">Sign-ins</span>
      <span aria-hidden className="flex h-5 w-[178px] flex-none items-end gap-0.5">
        {counts.map((n, i) => (
          <span
            key={i}
            className={`flex-1 rounded-[1px] ${n === 0 ? "bg-line" : "bg-muted"}`}
            style={{ height: n === 0 ? 2 : Math.max(3, Math.round((n / max) * TALL)) }}
          />
        ))}
      </span>
      <span className="min-w-0 flex-[1_1_200px] text-[14px] text-muted">
        {total} in 30 days, by {people} {people === 1 ? "person" : "people"}
      </span>
    </div>
  );
}

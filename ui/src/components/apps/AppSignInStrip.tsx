"use client";

import { Card } from "@/components/ui/Card";
import type { AppSignIns } from "@/lib/queries/useSignIns";

const DAYS = 30;
const TALL = 56;

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
 * An app's sign-ins per day, last 30 days, naming nobody (A-10 §7, `app-b`;
 * owner, 3 Nov: "1a with statistics"). One bar a day; a day with none is a
 * hairline, never a gap.
 */
export function AppSignInStrip({ name, usage }: { name: string; usage: AppSignIns }) {
  const byDay = new Map((usage.days ?? []).map((d) => [d.day, d.count]));
  const counts = lastDays().map((day) => byDay.get(day) ?? 0);
  const max = Math.max(1, ...counts);
  const total = counts.reduce((a, b) => a + b, 0);
  const people = usage.people_30d ?? 0;
  return (
    <Card>
      <div className="flex flex-col gap-3 px-[22px] pb-5 pt-[18px]">
        <div className="flex flex-col gap-1">
          <h2 className="type-card-title m-0">Sign-ins</h2>
          <p className="m-0 text-[13.5px] text-faint">Every sign-in to {name}, per day, last 30 days. Nobody is named.</p>
        </div>
        <span aria-hidden className="flex h-14 max-w-[560px] items-end gap-1">
          {counts.map((n, i) => (
            <span
              key={i}
              className={`flex-1 rounded-[2px] ${n === 0 ? "bg-line" : "bg-muted"}`}
              style={{ height: n === 0 ? 2 : Math.max(3, Math.round((n / max) * TALL)) }}
            />
          ))}
        </span>
        <p className="m-0 text-[14px] text-muted">
          {total} {total === 1 ? "sign-in" : "sign-ins"} in 30 days, by {people} {people === 1 ? "person" : "people"}.
        </p>
      </div>
    </Card>
  );
}

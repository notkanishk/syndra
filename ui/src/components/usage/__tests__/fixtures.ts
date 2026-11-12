import { addDays, today } from "@/components/insights/days";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { StatsMetric, TargetStats, UsageWindows } from "@/lib/queries/useStats";

/** Statistics fixtures: ring names and example.org only. Days are relative to today, in the makerspace's zone. */
export const GIB = 1024 ** 3;
export const ago = (n: number) => addDays(today(), -n);

/**
 * The windows the backend sends (handlers statsWindows, statsUsageFor): the
 * last 30 days, the 12 months from the first of the month 11 back, and for a
 * person kept `n` days (today − (n − 1) on) each start no earlier than that.
 */
export function windows(n?: number): UsageWindows {
  const [y, m] = today().split("-").map(Number);
  const [from30, from12m] = [ago(29), new Date(Date.UTC(y, m - 12, 1)).toISOString().slice(0, 10)];
  if (n === undefined) return { window_30d_from: from30, window_12m_from: from12m };
  const kept = ago(n - 1);
  return { kept_from: kept, window_30d_from: kept > from30 ? kept : from30, window_12m_from: kept > from12m ? kept : from12m };
}

export const homeBytes = (over: Partial<StatsMetric> = {}): StatsMetric => ({
  name: "volume.used_bytes",
  instrument: "gauge",
  unit: "bytes",
  label: "Home folder used",
  per_subject: true,
  against: "quota_field",
  watermark: `${ago(1)}T00:00:00+05:30`,
  series: [{ dimensions: {}, latest: { day: ago(1), value: 18.4 * GIB }, days: [{ day: ago(3), value: 18 * GIB }, { day: ago(1), value: 18.4 * GIB }] }],
  gaps: [],
  ...over,
});

export const sessions = (over: Partial<StatsMetric> = {}): StatsMetric => ({
  name: "volume.sessions",
  instrument: "counter",
  unit: "count",
  label: "Sessions",
  per_subject: true,
  dimensions: [{ name: "area", label: "Area" }],
  watermark: `${ago(1)}T00:00:00+05:30`,
  series: [
    { dimensions: { area: "ring" }, sum_30d: 7, sum_12m: 37, days: [{ day: ago(1), value: 3 }, { day: ago(0), value: 4 }], months: [{ month: ago(0).slice(0, 7), value: 7 }] },
    { dimensions: { area: "mat" }, sum_30d: 0, sum_12m: 3, days: [], months: [] },
  ],
  gaps: [],
  ...over,
});

/** Three unobserved days inside the ring series' window (start 8 days ago, end 5 days ago exclusive). */
export const GAP = { start: ago(8), end: ago(5), dimensions: { area: "ring" }, reason: "Auditing is off in this area" };

export const view = (usage: MyTargetView["usage"], over: Partial<MyTargetView> = {}): MyTargetView => ({
  target: "gym",
  name: "Training hall",
  entitled: true,
  account: { username: "rey_mysterio" },
  kind: "inventory",
  credential: { set: false },
  reachable: true,
  health: "online",
  usage,
  ...over,
});

export const targetStats = (over: Partial<TargetStats> = {}): TargetStats => ({
  target: "gym",
  usage: {
    reported: true,
    ...windows(),
    metrics: [
      sessions({
        series: [{ dimensions: { area: "ring" }, sum_30d: 61, sum_12m: 512, days: [{ day: ago(1), value: 9 }], months: [] }],
        gaps: [GAP],
        collected: { at: new Date(Date.now() - 4 * 60_000).toISOString(), unbound_accounts: 1, dropped: { series: 3 } },
      }),
    ],
  },
  retention: { subject_days: 400, set: false },
  ...over,
});

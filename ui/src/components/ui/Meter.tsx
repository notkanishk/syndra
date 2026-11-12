/**
 * The one meter: what is used against a limit that was read. One threshold
 * (90% is "nearly full", the limit itself is "full"), one size and one set of
 * tokens, so the same number looks the same on the member's card, the person
 * page, a status read and a statistic. Colour is never the only signal: past
 * the threshold the word is written beside the bar, and says the same to a
 * screen reader.
 */
export type MeterState = "ok" | "nearly" | "full";

export const NEARLY_FULL = 0.9;
export const METER_WORD = { nearly: "nearly full", full: "full" } as const;

export function meterState(value: number, limit: number): MeterState {
  if (limit <= 0) return "ok";
  return value >= limit ? "full" : value >= limit * NEARLY_FULL ? "nearly" : "ok";
}

export function Meter({
  value,
  limit,
  label,
  valueText,
  className = "",
}: {
  value: number;
  /** The limit, above 0: a limit of 0 is "no limit" and has no meter. */
  limit: number;
  label: string;
  /** What the bar says in words ("18.4 GiB of 20.0 GiB"); the state is added. */
  valueText?: string;
  className?: string;
}) {
  const state = meterState(value, limit);
  const word = state === "ok" ? null : METER_WORD[state];
  const pct = Math.min(100, (value / limit) * 100);
  return (
    <span className={`flex w-full max-w-[360px] items-center gap-2.5 ${className}`}>
      <span
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={limit}
        aria-valuenow={Math.min(value, limit)}
        aria-valuetext={[valueText, word].filter(Boolean).join(", ") || undefined}
        className="block h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-tint-3"
      >
        <span className={`block h-full rounded-full ${state === "ok" ? "bg-accent" : "bg-warn"}`} style={{ width: `${value > 0 ? Math.max(pct, 1) : 0}%` }} />
      </span>
      {word && <span className="shrink-0 whitespace-nowrap text-[13px] font-semibold text-warn-text">{word}</span>}
    </span>
  );
}

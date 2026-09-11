/**
 * The 3-tile stat row: label caps / big value / one-line sub.
 *
 * Shared so a stat reads the same on the Zitadel page and on a target's page
 * — one tile idiom rather than two that happen to look alike today and drift
 * apart the next time either screen is touched.
 *
 * `warn` is a deadline or a broken assumption; `accent` is a state somebody
 * chose on purpose (maintenance, a pause) and must never render as a fault;
 * `danger` is something that needs a decision now. Picking the wrong one
 * teaches an operator that the colour does not mean anything.
 */
export function StatCard({
  label,
  value,
  detail,
  tone = "neutral",
}: {
  label: string;
  value: string;
  detail: string;
  tone?: "neutral" | "accent" | "warn" | "danger";
}) {
  const valueTone =
    tone === "warn"
      ? "text-warn-text"
      : tone === "danger"
        ? "text-danger-text"
        : tone === "accent"
          ? "text-accent-text"
          : "";

  return (
    <div className="card min-w-[260px] flex-1 px-5 py-4">
      <div className="type-label mb-2">{label}</div>
      <div className={`font-display text-[24px] font-semibold ${valueTone}`}>{value}</div>
      <p className="mt-1.5 max-w-[42ch] text-[13px] leading-[1.5] text-muted">{detail}</p>
    </div>
  );
}

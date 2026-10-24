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
  href,
}: {
  label: string;
  value: string;
  detail: string;
  tone?: "neutral" | "accent" | "warn" | "danger";
  /**
   * The list this number counts (proposed/StatCard-href.md): the whole card is
   * the link, named by its label, with a 1px accent ring on hover and no arrow.
   * Pressed, --tint-3 is a second background layer over the card's own surface,
   * never in place of it (A-08 §5).
   */
  href?: string;
}) {
  const valueTone =
    tone === "warn"
      ? "text-warn-text"
      : tone === "danger"
        ? "text-danger-text"
        : tone === "accent"
          ? "text-accent-text"
          : "";

  const body = (
    <>
      <div className="type-label mb-2">{label}</div>
      <div className={`font-display text-[24px] font-semibold ${valueTone}`}>{value}</div>
      <p className="mt-1.5 max-w-[42ch] text-[13px] leading-[1.5] text-muted">{detail}</p>
    </>
  );
  const frame = "card min-w-[260px] flex-1 px-5 py-4";
  return href ? (
    <a
      href={href}
      aria-label={label}
      className={`${frame} block text-inherit no-underline motion-tint hover:shadow-[0_0_0_1px_var(--accent-line)] active:[background-image:linear-gradient(var(--tint-3),var(--tint-3))]`}
    >
      {body}
    </a>
  ) : (
    <div className={frame}>{body}</div>
  );
}

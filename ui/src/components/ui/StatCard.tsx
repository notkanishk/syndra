import { AdvancedLink } from "@/components/ui/AdvancedLink";

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
   * the link, named by its label, and no arrow. Under the hand it wears the edge
   * like any card (A-25 moment 2), never a flat ring all round. Pressed, --tint-3
   * is a second background layer over the card's own surface, never in place of
   * it (A-08 §5).
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
      <div data-figure className={`font-display text-figure ${valueTone}`}>{value}</div>
      <p className="mt-1.5 measure text-small leading-[1.5] text-muted">{detail}</p>
    </>
  );
  // A tile lights like any card, a link or not (A-25 moment 2: cards light, not only links): its
  // `card` frame carries the edge.
  const frame = "card min-w-[260px] flex-1 px-5 py-4";
  return href ? (
    <AdvancedLink
      href={href}
      aria-label={label}
      data-stat-tile
      className={`${frame} block text-inherit no-underline motion-tint active:[background-image:linear-gradient(var(--tint-3),var(--tint-3))]`}
    >
      {body}
    </AdvancedLink>
  ) : (
    <div data-stat-tile className={frame}>
      {body}
    </div>
  );
}

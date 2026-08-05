/**
 * The mark: a contained orb, and a miniature of the login's arch-and-orb.
 *
 * Three spans and no image — the geometry, the two masks and every colour
 * live in `globals.css` under `.syndra-mark`, so the rail and the favicon are
 * the same drawing rather than two that have to be kept in agreement, and the
 * light theme inverts it without this file knowing.
 *
 * `size` sets `--mark-size`; everything inside scales off it.
 */
export function SyndraMark({ size = 22, className = "" }: { size?: number; className?: string }) {
  return (
    <span
      aria-hidden
      className={`syndra-mark ${className}`}
      style={{ "--mark-size": `${size}px` } as React.CSSProperties}
    >
      <span className="syndra-mark-base" />
      <span className="syndra-mark-arc" />
      <span className="syndra-mark-dot" />
    </span>
  );
}

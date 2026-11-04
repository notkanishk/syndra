/**
 * The one fold control (RULES, A-17 G3): a 28px round chevron that turns when
 * its section opens. It is only the look: the caller's button around it carries
 * `aria-expanded` and the 44px touch floor. Six copies of this were written by
 * hand in two border tones before it was a part.
 */
export function FoldChevron({ open, className = "" }: { open: boolean; className?: string }) {
  return (
    <span
      aria-hidden
      data-fold-chevron
      className={`flex size-7 flex-none items-center justify-center rounded-pill border border-line-strong text-muted motion-turn ${
        open ? "rotate-90" : ""
      } ${className}`}
    >
      <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.6">
        <path d="M6 3.5 10.5 8 6 12.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </span>
  );
}

import Link from "next/link";

/**
 * "Somewhere else →": the one link that goes to another page.
 *
 * About seventeen were written by hand, at five sizes from 12.5px to 15px,
 * underlined on hover in some and brightened in others, the arrow sometimes
 * read aloud and sometimes not. One now: violet, semibold, 13.5px, the arrow
 * hidden from screen readers (the words already say where), and a 44px target
 * under a thumb. On hover the colour stays violet and the underline tints in
 * (`link-hover`, A-08 §2a); it no longer brightens. A link inside a sentence stays a plain link; this is for one
 * that stands on its own.
 */
export const ARROW_LINK =
  "inline-flex min-h-[44px] items-center gap-1 text-[13.5px] font-semibold text-accent-text link-hover desktop:min-h-0";

export function ArrowLink({
  href,
  children,
  className = "",
}: {
  href: string;
  /** The words only: the arrow is drawn here. */
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <Link href={href} className={`${ARROW_LINK} ${className}`}>
      {children}
      <span aria-hidden>→</span>
    </Link>
  );
}

import Link from "next/link";

/**
 * An address that matches no route.
 *
 * The design reply asks 404 to say one of two different things — *"This
 * existed and no longer does"* or *"There's nothing at this address"* —
 * because an operator chasing a link needs to know which. This boundary only
 * ever sees the second: nothing in the product calls `notFound()` for a
 * missing record, so a deleted person or bundle arrives as a 404 from the API
 * and is reported by that screen's own `ErrorState`, which knows what was
 * being looked for and can say so.
 *
 * So this page says the one thing it actually knows, rather than hedging
 * between two answers and being useless for both. If a screen ever starts
 * calling `notFound()` for a record, the other sentence belongs there, not
 * here.
 */
export default function NotFound() {
  return (
    <div className="flex min-h-[60dvh] items-center justify-center px-5">
      <div className="w-full max-w-[52ch] rounded-[18px] border border-line-strong bg-surface-2 px-6 py-7">
        <h1 className="type-card-title">There&rsquo;s nothing at this address.</h1>
        <p className="mt-2 text-[14.5px] leading-[1.55] text-muted">
          No page in Syndra has this address. If you followed a link, the link itself is wrong;
          nothing has been deleted. A person or a bundle that was removed would say so on its own
          page.
        </p>
        <Link
          href="/"
          className="mt-5 inline-flex min-h-[44px] items-center rounded-pill bg-accent-dense px-4 text-[13.5px] font-semibold text-accent-ink"
        >
          Go to the home page
        </Link>
      </div>
    </div>
  );
}

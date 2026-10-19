import { EmptyState } from "@/components/states";

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
    <EmptyState
      title="There’s nothing at this address."
      guidance="No page in Syndra has this address. If you followed a link, the link itself is wrong; nothing has been deleted. A person or a bundle that was removed would say so on its own page."
      action={{ label: "Go to the home page", href: "/" }}
    />
  );
}

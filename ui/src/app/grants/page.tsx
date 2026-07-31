import { permanentRedirect } from "next/navigation";

/**
 * /grants ceases to exist as a destination.
 *
 * It hosted two tabs doing unrelated jobs. "All grants" is absorbed: People
 * and role membership answer the same question with the access source
 * attached, which the flat ledger never had. Reconciliation had no other home,
 * so it moved into Review › Unexplained access as a second tab — and that is
 * where an old bookmark now lands, adjacent and legible rather than on a 404.
 */
export default function GrantsRedirect() {
  permanentRedirect("/governance/drift?tab=reconciliation");
}

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";

/**
 * S6 · Review › Drift. The highest-stakes screen in the product.
 *
 * One list, gathered by direction: access found in Zitadel that Syndra cannot
 * explain, and access Syndra gave that Zitadel no longer holds. (The Syndra and
 * Zitadel diff that used to be a second tab, relocated from the retired /grants
 * route, is these two cards; `?tab=reconciliation` links land on the list.)
 */
export default function UnexplainedAccessPage() {
  return <UnexplainedAccess />;
}

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";

/**
 * S6 · Review › Unexplained access. The highest-stakes screen in the product.
 *
 * Two tabs: the triage queue (access found in Zitadel that Syndra cannot
 * explain) and reconciliation (the Syndra ↔ Zitadel diff, relocated from the
 * retired /grants route).
 */
export default function UnexplainedAccessPage() {
  return <UnexplainedAccess />;
}

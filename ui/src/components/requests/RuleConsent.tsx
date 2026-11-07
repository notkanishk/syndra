"use client";

import { Button } from "@/components/ui/Button";
import type { PlanConsent } from "@/lib/usePlanConsent";

/**
 * The one consent rule for what an act also does (audit C1, C2, N5): giving,
 * approving, extending and adopting each wait for the act's plan
 * (`usePlanConsent`), and each stays held when the plan can't be read, with
 * the reason and a way to try again. What the automatic rules add is the
 * planner's sentence, one source, wherever it is said.
 */

/** What the person reads, and "Try again", when the plan couldn't be read. Nothing otherwise. */
export function RuleConsentFailed({
  consent,
  className,
}: {
  consent: Pick<PlanConsent, "failed" | "retry" | "unread">;
  className?: string;
}) {
  if (!consent.failed) return null;
  return (
    <span className={className ?? "flex w-full flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-muted"} role="status">
      <span>{consent.unread}</span>
      <Button variant="outline" size="sm" onClick={consent.retry}>
        Try again
      </Button>
    </span>
  );
}

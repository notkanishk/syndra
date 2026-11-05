"use client";

import { Button } from "@/components/ui/Button";
import { useRulePreview } from "@/lib/queries/useRulePreview";

/**
 * The one consent rule for what automatic rules also give (audit C1, C2, N5).
 *
 * Giving, approving and adopting all hand out more than the role named when a
 * rule follows it, so each one waits for the preview of that, and each one
 * stays held when the preview can't be read, with the reason and a way to try
 * again. Every surface takes its hold and its words from here, so they cannot
 * drift apart: Approve held on a failed preview while Give let it through with
 * a warning, and that was the same decision made two ways.
 */

/** Held while the preview loads. */
export const RULES_CHECKING = "Checking what automatic rules also give.";

/** Held when the preview can't be read: what the person is told, and the shorter reason under the button. */
export const RULES_UNREAD =
  "Syndra couldn't read the automatic rules, so it can't show what else this would give. Try again to read them.";
export const RULES_HELD = "Held until Syndra can read the automatic rules.";

/**
 * The preview and the hold that follows from it. `hold` is undefined when the
 * person may go ahead; otherwise it is the sentence to put under the disabled
 * button. `retry` reads the preview again.
 */
export function useRuleConsent(userId: string, projectId: string, roleKeys: string[], enabled = true) {
  const rules = useRulePreview(userId, projectId, roleKeys, enabled);
  // A retry after a failure keeps the query in error until it answers: it is
  // still a read in progress, so it is "checking", not "couldn't read".
  const checking = rules.isLoading || (rules.isError && rules.isFetching);
  const failed = rules.isError && !rules.isFetching;
  const hold = checking ? RULES_CHECKING : failed ? RULES_HELD : undefined;
  return {
    lines: (rules.data ?? []).map((row) => row.sentence),
    hold,
    checking,
    failed,
    retry: () => void rules.refetch(),
  };
}

/** What the person reads, and "Try again", when the preview couldn't be read. Nothing otherwise. */
export function RuleConsentFailed({
  consent,
  className,
}: {
  consent: Pick<ReturnType<typeof useRuleConsent>, "failed" | "retry">;
  className?: string;
}) {
  if (!consent.failed) return null;
  return (
    <span className={className ?? "flex w-full flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-muted"} role="status">
      <span>{RULES_UNREAD}</span>
      <Button variant="outline" size="sm" onClick={consent.retry}>
        Try again
      </Button>
    </span>
  );
}

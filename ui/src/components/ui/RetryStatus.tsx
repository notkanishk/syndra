import { StatusDot } from "@/components/ui/Badge";
import { memberRetryLine, retryLine, type RetryState } from "@/lib/retry";

/**
 * The one place a change that came back unanswered is shown: a Pending
 * changes row and a member's storage password alike (owner, 5 Nov 2026). Warn
 * while Syndra is still trying, danger once it has stopped, so a retry is
 * never silent and neither is its end.
 */
export function RetryStatus({
  retry,
  system = "Zitadel",
  own = false,
  className = "",
}: {
  retry: RetryState;
  system?: string;
  /** The person's own voice ("Not confirmed yet…"), for their own storage page. */
  own?: boolean;
  className?: string;
}) {
  const ended = retry.state === "failed" || retry.state === "waiting";
  return (
    <span className={`inline-flex items-center gap-2 text-[13px] ${className}`} role="status">
      <StatusDot tone={ended ? "danger" : "warn"} />
      <span className={ended ? "text-danger-text" : "text-warn-text"}>
        {own ? memberRetryLine(retry) : retryLine(retry, system)}
      </span>
    </span>
  );
}

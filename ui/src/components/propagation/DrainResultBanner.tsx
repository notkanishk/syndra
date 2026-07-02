"use client";

import type { DrainResult } from "@/lib/queries/usePropagation";

interface Props {
  result?: DrainResult;
}

const HALT_REASONS: Record<string, string> = {
  zitadel_offline: "Zitadel is offline — changes stay buffered until it is reachable again.",
  drain_in_progress: "Another drain is already running — try again once it finishes.",
  max_retries_exceeded: "A change exceeded the retry budget and halted the drain — inspect the worklist.",
};

/**
 * Summarizes the outcome of the last drain. The key case the operator must not
 * miss is `errored`: rows whose Zitadel outcome was decided but whose state
 * could not be persisted. Those stay in_flight (the worklist stays non-empty),
 * so without this banner a successful HTTP 200 would give no signal that a retry
 * is needed. Renders nothing until a drain has run.
 */
export function DrainResultBanner({ result }: Props) {
  if (!result) return null;

  const { applied, failed, requeued, errored, halted, reason } = result;

  if (errored > 0) {
    return (
      <div
        role="alert"
        className="rounded-card border border-error/40 bg-[color-mix(in_srgb,var(--error)_15%,transparent)] px-4 py-3 text-sm text-on-surface"
      >
        <strong>
          {errored} {errored === 1 ? "change" : "changes"} could not be recorded
        </strong>{" "}
        — the Zitadel call was made but the state write failed. These stay in flight; resume again to
        retry. ({applied} applied, {requeued} requeued, {failed} failed)
      </div>
    );
  }

  if (halted) {
    return (
      <div
        role="status"
        className="rounded-card border border-warning/40 bg-[color-mix(in_srgb,var(--warning)_15%,transparent)] px-4 py-3 text-sm text-on-surface"
      >
        {(reason && HALT_REASONS[reason]) ?? `Drain halted${reason ? `: ${reason}` : ""}.`}
      </div>
    );
  }

  return (
    <div
      role="status"
      className="rounded-card border border-outline-variant bg-surface-container-low/40 px-4 py-3 text-sm text-on-surface-variant"
    >
      Drain complete — <strong className="text-on-surface">{applied}</strong> applied
      {requeued > 0 && `, ${requeued} requeued`}
      {failed > 0 && `, ${failed} failed`}.
    </div>
  );
}

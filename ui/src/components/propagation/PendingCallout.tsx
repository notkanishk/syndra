"use client";

import { Button } from "@/components/ui/Button";

interface Props {
  count: number;
  reachable: boolean;
  dismissed: boolean;
  isResuming?: boolean;
  onResume: () => void;
  onDismiss: () => void;
}

/**
 * Amber, in-layout callout surfacing the count of MkAuth-mediated grant changes
 * buffered in the outbox and awaiting propagation to Zitadel. "Resume now"
 * drains the outbox; it is disabled when Zitadel is unreachable (the drain would
 * just halt). Renders nothing when there is nothing pending or the operator
 * dismissed it this session.
 */
export function PendingCallout({ count, reachable, dismissed, isResuming, onResume, onDismiss }: Props) {
  if (count <= 0 || dismissed) return null;
  return (
    <div
      role="status"
      className="flex items-center justify-between gap-4 rounded-card border border-tertiary/40 bg-[color-mix(in_srgb,var(--tertiary-container)_60%,transparent)] px-4 py-3"
    >
      <div className="flex items-center gap-3 text-on-surface">
        <span aria-hidden>⏱</span>
        <span className="text-sm">
          <strong>{count}</strong> {count === 1 ? "change" : "changes"} awaiting Zitadel
          {!reachable && <span className="ml-2 text-warning">— Zitadel offline</span>}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={onResume} disabled={!reachable} isPending={isResuming}>
          Resume now
        </Button>
        <button
          type="button"
          aria-label="Dismiss"
          onClick={onDismiss}
          className="px-1 text-on-surface/60 hover:text-on-surface"
        >
          ×
        </button>
      </div>
    </div>
  );
}

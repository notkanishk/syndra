"use client";

import { useOnline } from "@/lib/useOnline";

/**
 * No network, said as its own thing.
 *
 * Deliberately not the degraded banner, and deliberately not amber. Degraded
 * is a broken assumption — the directory fell back and the numbers on screen
 * are invented — and amber is the colour this product reserves for exactly
 * that. Offline is not a broken assumption: what is on screen is true, it is
 * simply not being kept up to date. Neutral surface, stated plainly.
 *
 * Pinned under the status bar and not dismissible, because a dismissed banner
 * and a working network look identical, and the difference decides whether an
 * operator trusts what they are reading.
 *
 * There is no retry control and no client-side queue. A queue in the browser
 * would be a second ledger nobody can inspect, in a product whose whole
 * argument is that Syndra decides and records; and a Retry button next to a
 * banner that already disappears the moment the network returns is a control
 * for something that happens by itself.
 */
export function OfflineBanner() {
  const online = useOnline();
  if (online) return null;

  return (
    <div
      role="status"
      className="flex items-start gap-3.5 border-b border-line-strong bg-surface-2 px-4 py-3.5 tablet:px-[26px]"
    >
      <span
        aria-hidden
        className="mt-[3px] h-2.5 w-2.5 flex-none rounded-pill border border-muted"
      />
      <div>
        <div className="text-[14.5px] font-semibold">No network connection.</div>
        <p className="mt-1 max-w-[70ch] text-[14px] leading-[1.55] text-muted">
          What you see is the last thing Syndra loaded, and it may be out of date. You cannot
          make changes until the connection is back. Anything you try is stopped before it is
          sent, so nothing is sent twice.
        </p>
      </div>
    </div>
  );
}

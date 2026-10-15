"use client";

import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/components/ui/Unfold";
import { useOnline } from "@/lib/useOnline";

/**
 * No network, said as its own thing.
 *
 * Deliberately not the degraded Notice, and deliberately not amber. Degraded
 * is a broken assumption, and amber is the colour this product reserves for
 * exactly that. Offline is not a broken assumption: what is on screen is true,
 * it is simply not being kept up to date. Neutral, hollow dot, stated plainly.
 *
 * In the content column, above the page (Motion 6a): it opens to its height
 * and pushes the page down, and closes the gap when the network returns. Not
 * dismissible, because a dismissed Notice and a working network look
 * identical, and the difference decides whether an operator trusts what they
 * are reading.
 *
 * There is no retry control and no client-side queue. A queue in the browser
 * would be a second ledger nobody can inspect, in a product whose whole
 * argument is that Syndra decides and records; and a Retry button beside a
 * Notice that already leaves the moment the network returns is a control for
 * something that happens by itself.
 */
export function OfflineBanner() {
  const online = useOnline();

  return (
    <Unfold open={!online}>
      <div className="pb-4">
        <Notice tone="neutral" title="No network connection.">
          What you see is the last thing Syndra loaded, and it may be out of date. You can&apos;t
          make changes until the connection is back. Anything you try is stopped before it is
          sent, so nothing is sent twice.
        </Notice>
      </div>
    </Unfold>
  );
}

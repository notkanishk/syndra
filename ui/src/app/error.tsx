"use client";

import { useEffect } from "react";

import { CopyableValue } from "@/components/ui/CopyableValue";

/**
 * When the app itself fails.
 *
 * Everything a *request* can do wrong already has a home: `ErrorState` names
 * the failed read, confirms nothing was changed, and offers a retry that means
 * something. This is the other kind — the render itself threw — and until now
 * there was nothing here at all, which on a phone is a white screen with no
 * identifier, no way back, and no evidence anything happened.
 *
 * **No "Try again".** Next hands this boundary a `reset()` and the convention
 * is to wire it to a button, but a render that threw on the data it was given
 * throws again on the same data, and a button that repeats a failure while
 * looking like a remedy is worse than no button. Retrying a *fetch* is already
 * offered where retrying works. What is offered here is the way out and the
 * identifier somebody can quote.
 *
 * The digest is Next's own hash of the error, the only handle that survives
 * into production where the message is stripped. It is rendered as a copy row
 * for the same reason request ids are: the alternative is somebody
 * transcribing a hash by eye from a phone screen.
 */
export default function AppError({ error }: { error: Error & { digest?: string } }) {
  useEffect(() => {
    // The console is where this is recoverable in development; in production
    // the digest is the join key to the server log.
    console.error("Syndra failed to render:", error);
  }, [error]);

  return (
    <div className="flex min-h-[60dvh] items-center justify-center px-5">
      <div className="w-full max-w-[52ch] rounded-[18px] border border-danger-line bg-surface-2 px-6 py-7">
        <h1 className="type-card-title">This page could not be shown.</h1>
        <p className="mt-2 text-[14.5px] leading-[1.55] text-muted">
          Something in the page itself failed, not something you did. Nothing was changed:
          this happened while showing you a screen, not while saving anything.
        </p>

        {error.digest ? (
          <div className="mt-4">
            <div className="mb-1 type-label">Reference</div>
            <CopyableValue label="Reference" value={error.digest} />
            <p className="mt-1 text-[12.5px] text-faint">Quote this if you ask for help.</p>
          </div>
        ) : null}

        {/* A plain anchor rather than a Link, deliberately: a soft navigation
            re-renders the same client tree, and if what threw was in a layout
            or a provider it throws again on arrival. A full document load is
            the one way back that does not depend on the thing that broke. */}
        {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
        <a
          href="/"
          className="mt-5 inline-flex min-h-[44px] items-center rounded-pill bg-accent-dense px-4 text-[13.5px] font-semibold text-accent-ink"
        >
          Go to the home page
        </a>
      </div>
    </div>
  );
}

"use client";

import { useEffect } from "react";

import { ARROW_LINK } from "@/components/ui/ArrowLink";
import { Notice } from "@/components/ui/Notice";
import { NOTHING_CHANGED } from "@/lib/outcome";

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

  // The shell is still standing, so this is said the way every failure is:
  // the danger Notice in the content column, ending "Nothing was changed.",
  // the reference as a copy row, and the way home as a violet text link.
  return (
    <div className="flex max-w-[76ch] flex-col gap-4">
      <Notice
        tone="danger"
        blocking
        title="This page couldn't be shown."
        reference={error.digest || undefined}
      >
        Something in the page itself failed, not something you did. It happened while showing you
        a screen, not while saving anything. {NOTHING_CHANGED}
      </Notice>

      {/* A plain anchor rather than a Link, deliberately: a soft navigation
          re-renders the same client tree, and if what threw was in a layout
          or a provider it throws again on arrival. A full document load is
          the one way back that does not depend on the thing that broke. */}
      {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
      <a href="/" className={`${ARROW_LINK} self-start`}>
        Go to the home page <span aria-hidden>→</span>
      </a>
    </div>
  );
}

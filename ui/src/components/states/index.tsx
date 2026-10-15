"use client";

import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect } from "react";

import { ApiError } from "@/lib/api-client";
import { Notice } from "@/components/ui/Notice";
// The prose a failure wears lives in one place, shared with every surface that
// reports a mutation. A read that fails and a write that fails were describing
// the same 403 in two files until this import existed.
import { NOTHING_CHANGED, describeFailure, sentence } from "@/lib/outcome";
import { useUiView } from "@/lib/ui-view";

/**
 * Four states, every list view. Not three — degraded is real and specific and
 * lives in DegradedBanner.
 *
 * These three are the per-list set: a skeleton at the real row height, an
 * empty state that names what is absent, and an error that names the failed
 * thing and confirms nothing changed.
 */

/**
 * Row-shaped skeletons at the real row height (avatar circle + text bars,
 * descending opacity), so nothing jumps when data lands.
 */
export function RowSkeleton({
  rows = 4,
  avatar = true,
  label = "Loading",
}: {
  rows?: number;
  avatar?: boolean;
  label?: string;
}) {
  return (
    <div className="flex flex-col gap-3 p-4" aria-busy="true" aria-label={label}>
      {Array.from({ length: rows }).map((_, index) => (
        <div
          key={index}
          className="flex items-center gap-3"
          // Descending opacity: the list reads as continuing past the fold
          // rather than as four items that are about to arrive.
          style={{ opacity: 1 - index * 0.18 }}
        >
          {avatar && <span className="skeleton-bar h-[28px] w-[28px] flex-none rounded-pill" />}
          <span className="skeleton-bar h-[10px] flex-1" />
          <span className="skeleton-bar h-[10px] w-16 flex-none" />
        </div>
      ))}
    </div>
  );
}

/**
 * Empty — a sentence naming what is absent, one line of guidance, one link to
 * the next move. No illustration, no "you're all caught up!".
 *
 * `resolved` separates the two very different things an empty list can mean.
 * A triage queue with nothing in it is *resolved*: work existed, and none of
 * it needs you. A people list with nothing in it is merely *absent*: nobody
 * has been added yet, which is not good news, it is just news. Only the first
 * earns the healthy dot, and it is the caller's assertion rather than
 * something inferred from a zero — the component cannot tell them apart.
 */
export function EmptyState({
  title,
  guidance,
  action,
  resolved = false,
}: {
  title: string;
  guidance?: string;
  action?: { label: string; href: string } | { label: string; onClick: () => void };
  resolved?: boolean;
}) {
  return (
    <div role="status" className="flex flex-col justify-center gap-2.5 px-6 py-8">
      <div className="flex items-center gap-2.5">
        {resolved && (
          <span aria-hidden className="h-2 w-2 flex-none rounded-pill bg-healthy" />
        )}
        <div className="type-empty-title">{title}</div>
      </div>
      {guidance && <p className="max-w-[60ch] text-[14px] text-muted">{guidance}</p>}
      {action &&
        ("href" in action ? (
          <Link
            href={action.href}
            className="mt-1 inline-flex min-h-[44px] items-center self-start text-[13.5px] font-semibold text-accent-text hover:underline"
          >
            {action.label} <span aria-hidden="true">→</span>
          </Link>
        ) : (
          <button
            type="button"
            onClick={action.onClick}
            className="mt-1 inline-flex min-h-[44px] items-center self-start text-[13.5px] font-semibold text-accent-text hover:underline"
          >
            {action.label} <span aria-hidden="true">→</span>
          </button>
        ))}
    </div>
  );
}

/**
 * Error — names the failed thing, confirms nothing was changed, offers a
 * retry, and carries a request id the operator can paste into a message.
 *
 * "Nothing was changed" is not reassurance copy. On a screen about who can
 * operate a laser cutter, the first question after a failed load is whether
 * the failure did something.
 */
export function ErrorState({
  title,
  error,
  onRetry,
  /**
   * True when the error already sits inside a card — it then drops its own
   * surface rather than drawing a second bordered box inside the first.
   */
  bare = false,
}: {
  title: string;
  error: unknown;
  onRetry?: () => void;
  bare?: boolean;
}) {
  // Zitadel never read: the degraded Notice above the page already says so,
  // and a second, red one here would call it a failure of this list (N23).
  if (isNeverRead(error)) return <NeverRead />;

  const requestId = error instanceof ApiError ? error.details?.request_id : undefined;

  // The danger Notice (proposed/Notice.md), in the place of what failed to
  // load. Inside a card it keeps the card's inset rather than drawing its own
  // edge against the card's.
  return (
    <div className={bare ? "row-divider px-4 py-4 tablet:px-5" : ""}>
      <Notice
        tone="danger"
        blocking
        title={title}
        action={onRetry ? { label: "Try again", onClick: onRetry } : undefined}
        reference={typeof requestId === "string" ? requestId : undefined}
      >
        {`${sentence(describeFailure(error))} ${NOTHING_CHANGED}`}
      </Notice>
    </div>
  );
}

/** A read Zitadel has never answered: there is nothing to show, not a failure. */
export function isNeverRead(error: unknown): boolean {
  return error instanceof ApiError && error.code === "DIRECTORY_UNREACHABLE";
}

/**
 * In the place of a read Zitadel has never answered (N23). An operator has
 * the degraded Notice above the page and sees nothing more here; a member,
 * who doesn't, gets one neutral line. Either way the page asks again every
 * minute on its own, so it fills in when Zitadel answers.
 */
function NeverRead() {
  const { isOperator } = useUiView();
  const client = useQueryClient();
  useEffect(() => {
    const again = setInterval(
      () => client.invalidateQueries({ predicate: (q) => q.state.status === "error" }),
      60_000,
    );
    return () => clearInterval(again);
  }, [client]);

  if (isOperator) return null;
  return (
    <div className="px-4 py-4 tablet:px-5">
      <Notice tone="neutral" title="Syndra can't reach the sign-in system right now.">
        Nothing about your access has changed. This page tries again on its own.
      </Notice>
    </div>
  );
}

/**
 * The standard list wrapper: one component so no view can quietly skip a
 * state. Loading and error take precedence over empty, because an empty list
 * that is actually a failed request is the most expensive lie a list can tell.
 */
export function ListStates({
  isLoading,
  error,
  isEmpty,
  onRetry,
  errorTitle,
  empty,
  skeleton,
  children,
}: {
  isLoading: boolean;
  error: unknown;
  isEmpty: boolean;
  onRetry?: () => void;
  errorTitle: string;
  empty: React.ReactNode;
  skeleton?: React.ReactNode;
  children: React.ReactNode;
}) {
  if (isLoading) return <>{skeleton ?? <RowSkeleton />}</>;
  // Never read (N23): no skeleton, no empty state, no rows.
  if (isNeverRead(error)) return <NeverRead />;
  if (error) return <ErrorState title={errorTitle} error={error} onRetry={onRetry} bare />;
  if (isEmpty) return <>{empty}</>;
  // `arrive`, applied at the one place every list in the product already
  // passes through — so a new list gets it by being a list, and no view can
  // quietly opt out of it any more than it can skip an empty state.
  //
  // `contents` keeps the wrapper out of the layout entirely: this sits inside
  // flex and grid parents everywhere, and a real box here would collapse the
  // gaps between rows.
  return <div className="contents arrive-list">{children}</div>;
}

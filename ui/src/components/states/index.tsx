"use client";

import Link from "next/link";

import { ApiError } from "@/lib/api-client";
import { Button } from "@/components/ui/Button";
import { CopyableValue } from "@/components/ui/CopyableValue";
// The prose a failure wears lives in one place, shared with every surface that
// reports a mutation. A read that fails and a write that fails were describing
// the same 403 in two files until this import existed.
import { NOTHING_CHANGED, describeFailure, sentence } from "@/lib/outcome";

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
            className="mt-1 text-[13.5px] font-semibold text-accent-text hover:underline"
          >
            {action.label} →
          </Link>
        ) : (
          <button
            type="button"
            onClick={action.onClick}
            className="mt-1 self-start text-[13.5px] font-semibold text-accent-text hover:underline"
          >
            {action.label} →
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
  const detail = describeFailure(error);
  const requestId = error instanceof ApiError ? error.details?.request_id : undefined;

  return (
    <div
      role="alert"
      className={
        bare
          ? "row-divider flex flex-col gap-2.5 px-6 py-7"
          : "flex flex-col gap-2.5 rounded-card border border-danger-line bg-surface-1 px-6 py-7"
      }
    >
      <div className={`type-empty-title ${bare ? "text-danger-text" : ""}`}>{title}</div>
      <p className="text-[14px] text-muted">{`${sentence(detail)} ${NOTHING_CHANGED}`}</p>
      {/* The shared control, not a hand-rolled pill: a browser pass found this
          one rendering at 32px on a phone while every `Button` in the product
          cleared 44, because the floor lives in `buttonClasses` and this had
          walked around it. */}
      <div className="mt-1.5 flex flex-wrap items-center gap-2">
        {onRetry && (
          <Button variant="ghost" size="sm" onClick={onRetry}>
            Try again
          </Button>
        )}
        {/* A labelled copy row rather than bare mono: an operator pasting this
            into a message needs to be able to say what it is, and it is the
            one thing here that gets the failure looked at. */}
        {requestId && <CopyableValue value={requestId} label="Request id" className="mt-1" />}
      </div>
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

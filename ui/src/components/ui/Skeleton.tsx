"use client";

import React, { useState } from "react";

type SkeletonProps = React.HTMLAttributes<HTMLDivElement>;

/**
 * Pulse-animated placeholder block. Use in `space-y-3` stacks alongside
 * representative geometry (heights and widths matching the eventual
 * content) so the page layout doesn't shift when data arrives.
 */
export function Skeleton({ className = "", ...props }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      className={`skeleton-bar ${className}`}
      {...props}
    />
  );
}

/**
 * What a thing that was a skeleton wears when it lands (S7): `tint-in`, so the
 * swap fades rather than snaps. Only if a skeleton was drawn first; a name that
 * was there on the first render is simply there. `loading` is whether it is
 * still a skeleton.
 */
export function useTintAfterSkeleton(loading: boolean): string {
  const [waited, setWaited] = useState(loading);
  if (loading && !waited) setWaited(true);
  return waited && !loading ? "tint-in" : "";
}

/** Convenience: a stack of card-sized skeletons. */
export function SkeletonCardList({ count = 3 }: { count?: number }) {
  return (
    <div className="space-y-3" aria-busy="true" aria-label="Loading this list">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="rounded-block border border-line p-4 space-y-2">
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-4 w-2/3" />
          <div className="flex gap-2">
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-5 w-24" />
          </div>
        </div>
      ))}
    </div>
  );
}

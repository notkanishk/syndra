"use client";

import React from "react";

import { CountChip } from "@/components/ui/Badge";

/**
 * One region of the target page (design T1).
 *
 * The page carries eleven answers and used to carry them as eleven peers, in
 * the order they were added. Four regions is the structure that replaced that,
 * and this is the seam between them: a heading, its count, and one sentence of lede.
 *
 * Every region keeps its seat, always, whatever its count. A region that
 * appeared with its first finding would be structure moving in response to
 * data — and the page would then read differently depending on whether
 * somebody's access happened to be disputed, which is the one difference this
 * page has to make legible without moving anything.
 */
export function Region({
  title,
  count,
  lede,
  id,
  children,
}: {
  title: string;
  /** Omit where the region is not a list of things. `null` is "could not read". */
  count?: number | null;
  lede?: React.ReactNode;
  /** Anchor, so the touch index can jump to it. */
  id: string;
  children: React.ReactNode;
}) {
  return (
    <section id={id} aria-labelledby={`${id}-heading`} className="grid scroll-mt-4 gap-(--region-head-gap)">
      <div className="grid gap-1.5 tablet:gap-1">
        <div className="flex flex-wrap items-center gap-2.5">
          <h2 id={`${id}-heading`} className="type-section-title">
            {title}
          </h2>
          {count !== undefined && <CountChip n={count} />}
        </div>
        {lede && (
          <p data-prose className="measure text-body text-muted">
            {lede}
          </p>
        )}
      </div>
      <div className="grid gap-4">{children}</div>
    </section>
  );
}

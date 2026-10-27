"use client";

import { useState, type ReactNode } from "react";

import { Button } from "@/components/ui/Button";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { pageSettled } from "@/components/ui/ArriveList";
import { UnfoldOnArrival, useUnfoldOpening } from "@/components/ui/Unfold";

/**
 * The one way Syndra says something is wrong with what you are looking at
 * (proposed/Notice.md): data is stale or paused (warn), a read failed
 * (danger), or the system is out of reach and nothing is in doubt (neutral).
 *
 * One per card. `blocking` makes it an alert: the thing it is about can't be
 * used until it clears. Otherwise it is a status, read when reached.
 */
const TONES = {
  warn: { box: "border-warn-line bg-warn-soft", dot: "bg-warn", title: "text-warn-text" },
  danger: { box: "border-danger-line bg-danger-soft", dot: "bg-danger", title: "text-danger-text" },
  neutral: {
    box: "border-line-strong bg-surface-2",
    dot: "border border-muted bg-transparent",
    title: "text-ink",
  },
} as const;

/**
 * True once the app has painted. A Notice that is part of its page's first
 * paint (the app's, or a page reached by a link) arrives with its page; one
 * that mounts after it (a read that failed, a state
 * that went stale) opens its space by Unfold and tints in behind it, still and
 * without a rise, so what is below settles rather than jumps (A-08 §5). It
 * leaves by fading, then folding: a caller that can drop it while the page is
 * open wraps it in `<Unfold open={…}>`, which also opens it, so one that
 * mounts as that Unfold opens is not opened twice. One that mounts later
 * inside an Unfold already open opens itself. Two frames after this module loads is past first paint and
 * before any later mount.
 */
let painted = false;
if (typeof window !== "undefined") {
  requestAnimationFrame(() =>
    requestAnimationFrame(() => {
      painted = true;
    }),
  );
}

export function Notice({
  tone,
  title,
  children,
  live = false,
  blocking = false,
  action,
  reference,
  className = "",
}: {
  tone: keyof typeof TONES;
  title: string;
  children: ReactNode;
  /** The dot breathes while the condition is still going on. */
  live?: boolean;
  blocking?: boolean;
  action?: { label: string; onClick: () => void };
  /** A request id, shown copyable so it can be quoted. */
  reference?: string;
  className?: string;
}) {
  const t = TONES[tone];
  const opening = useUnfoldOpening();
  const [late] = useState(() => painted && pageSettled() && !opening);
  const box = (
    <div
      role={blocking ? "alert" : "status"}
      className={`flex items-start gap-3.5 rounded-panel border px-5 py-4 ${t.box} ${className}`}
    >
      <span
        aria-hidden
        className={`mt-[7px] h-2 w-2 flex-none rounded-pill ${t.dot} ${live ? "breathe" : ""}`}
      />
      <div className="flex min-w-0 flex-col gap-1.5">
        <span className={`font-display text-[19px] font-semibold leading-tight ${t.title}`}>{title}</span>
        <div className="max-w-[72ch] text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">
          {children}
        </div>
        {(action || reference) && (
          <div className="mt-1.5 flex flex-wrap items-center gap-3">
            {action && (
              <Button variant="outline" size="sm" onClick={action.onClick}>
                {action.label}
              </Button>
            )}
            {reference && <CopyableValue value={reference} label="Reference" />}
          </div>
        )}
      </div>
    </div>
  );
  return late ? <UnfoldOnArrival>{box}</UnfoldOnArrival> : box;
}

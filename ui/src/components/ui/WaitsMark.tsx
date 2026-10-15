"use client";

import { useRef, useState } from "react";

import { AnchoredPanel } from "@/components/ui/Anchored";

/**
 * The Waits-for-you mark (N18, 2c): a half-filled dot after a role that always
 * waits for the operator. Read-only everywhere; it is set only in Roles that
 * wait. Explained on hover and on focus, so a keyboard reaches the same words
 * a pointer does. `withLabel` says it in words where the row has room.
 */
export function WaitsMark({
  withLabel = false,
  className = "",
}: {
  withLabel?: boolean;
  className?: string;
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const [open, setOpen] = useState(false);
  const show = () => setOpen(true);
  const hide = () => setOpen(false);

  return (
    <span
      className={`inline-flex items-center gap-1.5 align-middle ${className}`}
    >
      <span
        ref={anchor}
        role="img"
        aria-label="Waits for you"
        aria-describedby={open ? "waits-mark-tip" : undefined}
        tabIndex={0}
        onMouseEnter={show}
        onMouseLeave={hide}
        onFocus={show}
        onBlur={hide}
        className="inline-block size-[9px] flex-none rounded-pill border-[1.5px] border-muted bg-[linear-gradient(90deg,var(--muted)_50%,transparent_50%)]"
      />
      {withLabel && (
        <span className="text-[13px] text-muted">Waits for you</span>
      )}
      <AnchoredPanel
        anchor={anchor}
        open={open}
        width={260}
        expectHeight={64}
        id="waits-mark-tip"
      >
        <span
          role="tooltip"
          className="tint-in shadow-popover block rounded-inner border border-line-strong bg-surface-1 px-3 py-2 text-[12.5px] leading-[1.45] text-ink"
        >
          Waits for you: when someone gains or loses this role, the change waits
          for you to send it.
        </span>
      </AnchoredPanel>
    </span>
  );
}

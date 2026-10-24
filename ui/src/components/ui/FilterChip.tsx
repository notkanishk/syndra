"use client";

import React, { useEffect, useRef, useState } from "react";

import { TINT_MS, stillMotion } from "@/lib/motion";

/**
 * A filter that is in force, with the way out attached to it.
 *
 * Two copies of this existed — one on People as a component, one written inline
 * on the audit log — and they had already agreed to be wrong in the same way:
 * the ✕ was a 20px target, which is half the floor, on a control whose whole
 * job is to be the way back out of a view somebody arrived at by link. A filter
 * you cannot clear on a phone is a filter that has silently become the state of
 * the page.
 *
 * The chip is not a `Button`: it is a statement with a control inside it, and
 * making the whole thing pressable would mean the label is a click target that
 * clears something. The ✕ carries the floor instead.
 */
export function FilterChip({
  children,
  onClear,
  clearLabel,
}: {
  children: React.ReactNode;
  onClear: () => void;
  /** What the ✕ does, for anybody who cannot see which chip it sits in. */
  clearLabel: string;
}) {
  // A chip tints in where it appears and tints out when its own ✕ clears it:
  // the clear runs one tint later, once the chip has gone. (A chip that goes
  // because another control changed the filter is unmounted by its parent and
  // simply goes.)
  const [leaving, setLeaving] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  function clear() {
    if (leaving) return;
    if (stillMotion()) return onClear();
    setLeaving(true);
    timer.current = setTimeout(onClear, TINT_MS);
  }
  return (
    <span
      className={`${leaving ? "tint-out pointer-events-none" : "tint-in"} inline-flex items-center gap-1 self-start rounded-pill bg-tint-2 py-1.5 pl-4 pr-1 text-[13.5px]`}
    >
      <span>{children}</span>
      <button
        type="button"
        onClick={clear}
        aria-label={clearLabel}
        className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-pill font-semibold text-muted control hover:text-ink desktop:min-h-0 desktop:min-w-0 desktop:px-2 desktop:py-0.5"
      >
        ✕
      </button>
    </span>
  );
}

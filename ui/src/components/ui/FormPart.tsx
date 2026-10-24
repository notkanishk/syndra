"use client";

import { useEffect, useState, type ReactNode } from "react";

import { LeavingRow, type LeaveProps } from "@/components/ui/LeavingRows";
import { Unfold } from "@/components/ui/Unfold";

/**
 * A row of a form or an editor (Motion v2, form parts). Added: it opens to its
 * own height, its contents tinting in one tint behind. Removed: it fades first,
 * then its gap settles. The one way a form gains or loses a part, so nothing
 * below it jumps.
 *
 * `appear` is for a part added while the reader is here; the parts a form opens
 * with are simply there. Removal is `useLeavingRows`' (S5) with no `listProps`
 * on the form, so the gap never waits for the pointer: the pointer is on the
 * Remove button that was just pressed.
 */
export function FormPart({
  appear = false,
  children,
  fresh: _fresh, // a part opens by its own Unfold; a second fade over it would double
  ...leave
}: LeaveProps & { appear?: boolean; children: ReactNode }) {
  const [open, setOpen] = useState(!appear);
  useEffect(() => {
    if (!open) setOpen(true);
  }, [open]);
  return (
    <LeavingRow {...leave}>
      <Unfold open={open}>{children}</Unfold>
    </LeavingRow>
  );
}

/**
 * Which parts were added after the form opened. `ready` is false until the
 * first read has landed, so what it opens with is not counted as added.
 */
export function useAddedKeys(keys: readonly string[], ready = true): ReadonlySet<string> {
  const [base, setBase] = useState<ReadonlySet<string> | null>(null);
  if (ready && base === null) setBase(new Set(keys));
  return new Set(base ? keys.filter((key) => !base.has(key)) : []);
}

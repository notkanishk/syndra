"use client";

import type { ReactNode } from "react";

import { Unfold } from "@/lib/motion";
import { useIsAdvanced } from "@/lib/ui-view";

/**
 * A part of a shared page that only Advanced shows: the machine's words (ids, a lifecycle control,
 * what an add-on also says). It opens to its height when the view is switched and folds away again,
 * like every other block that comes and goes (`Unfold`).
 *
 * It hides a part, never moves one: the parts around it keep their order, so an in-page jump to a
 * section that stays in Basic lands in the same place either way (basic-advanced-ia, §1). `gap` is
 * the row gap the parent puts around it, handed back so the page does not hop by one gap.
 * `className` goes on the region itself, so in Basic, where it renders nothing, no empty box is left
 * in a flex row to force a line break and a gap.
 */
export function AdvancedOnly({ children, gap, className }: { children: ReactNode; gap?: number; className?: string }) {
  return (
    <Unfold open={useIsAdvanced()} gap={gap} className={className}>
      {children}
    </Unfold>
  );
}

"use client";

import { useId } from "react";

import { Segmented } from "@/components/ui/Select";
import { useUiView } from "@/lib/ui-view";

/**
 * The Basic / Advanced switch.
 *
 * Segmented sm, like the chime toggle (A-04 §7.8): both destinations stay
 * legible and the current one is unmistakable across a noisy room, and it is
 * never a dropdown. It is the one control here that changes what the whole
 * application shows, so it keeps the touch floor Segmented carries below the
 * desktop breakpoint.
 *
 * Not rendered for members at all — not greyed, not present-but-403.
 */
/** What each view shows, said once where the switch is met. */
export const VIEW_HINT: Record<"basic" | "advanced", string> = {
  basic: "Basic view: people, projects, roles and requests.",
  advanced: "Advanced view: adds bundles, automatic rules, review lists and connected systems.",
};

export const VIEW_EXPLANATION = `${VIEW_HINT.basic} ${VIEW_HINT.advanced}`;

export const VIEW_OPTIONS = [
  { value: "basic", label: "Basic", title: VIEW_HINT.basic },
  { value: "advanced", label: "Advanced", title: VIEW_HINT.advanced },
] as const;

export function ViewSwitch({ fill = false }: { fill?: boolean }) {
  const { view, setView, isOperator } = useUiView();
  // Two of these can be mounted at once (the top bar and the phone sheet), so
  // the sentence they point at is each its own.
  const explanationId = useId();
  if (!isOperator) return null;

  return (
    <>
      <span id={explanationId} className="sr-only">
        {VIEW_EXPLANATION}
      </span>
      <Segmented
        size="sm"
        fill={fill}
        label="Basic or Advanced view"
        describedBy={explanationId}
        options={[...VIEW_OPTIONS]}
        value={view}
        onChange={setView}
      />
    </>
  );
}

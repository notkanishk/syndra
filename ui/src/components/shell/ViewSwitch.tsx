"use client";

import { useUiView } from "@/lib/ui-view";

import { PILL } from "@/components/ui/Button";

/**
 * The Basic / Advanced switch.
 *
 * A two-state segmented pill, never a dropdown: both destinations stay
 * legible and the current one is unmistakable across a noisy room.
 *
 * Not rendered for members at all — not greyed, not present-but-403.
 */
/** What each view shows, said once where the switch is met. */
export const VIEW_HINT: Record<"basic" | "advanced", string> = {
  basic: "Basic view: people, projects, roles and requests.",
  advanced: "Advanced view: adds bundles, automatic rules, review lists and connected systems.",
};

export const VIEW_EXPLANATION = `${VIEW_HINT.basic} ${VIEW_HINT.advanced}`;

export function ViewSwitch() {
  const { view, setView, isOperator } = useUiView();
  if (!isOperator) return null;

  return (
    <div
      role="radiogroup"
      aria-label="Basic or Advanced view"
      aria-describedby="view-switch-explanation"
      className="flex rounded-pill bg-tint-2 p-1"
    >
      <span id="view-switch-explanation" className="sr-only">
        {VIEW_EXPLANATION}
      </span>
      {(["basic", "advanced"] as const).map((option) => {
        const active = view === option;
        return (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={active}
            title={VIEW_HINT[option]}
            onClick={() => setView(option)}
            // 44px up to the desktop breakpoint, because everything below it
            // is a touch device and this is a primary control — it changes
            // what the whole application shows. The board draws it at 34,
            // which is ten under the floor and the one place a drawn value
            // could not be followed.
            className={`rounded-pill motion-tint ${PILL.md} ${
              active
                ? "bg-accent-dense font-semibold text-accent-ink"
                : "text-muted hover:text-ink"
            }`}
          >
            {option === "basic" ? "Basic" : "Advanced"}
          </button>
        );
      })}
    </div>
  );
}

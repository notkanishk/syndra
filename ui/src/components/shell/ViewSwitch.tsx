"use client";

import { useUiView } from "@/lib/ui-view";

/**
 * The Basic / Advanced switch.
 *
 * A two-state segmented pill, never a dropdown: both destinations stay
 * legible and the current one is unmistakable across a noisy room.
 *
 * Not rendered for members at all — not greyed, not present-but-403.
 */
export function ViewSwitch() {
  const { view, setView, isOperator } = useUiView();
  if (!isOperator) return null;

  return (
    <div
      role="radiogroup"
      aria-label="View"
      className="flex rounded-pill bg-tint-2 p-1"
    >
      {(["basic", "advanced"] as const).map((option) => {
        const active = view === option;
        return (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => setView(option)}
            // 44px up to the desktop breakpoint, because everything below it
            // is a touch device and this is a primary control — it changes
            // what the whole application shows. The board draws it at 34,
            // which is ten under the floor and the one place a drawn value
            // could not be followed.
            className={`min-h-[44px] rounded-pill px-4 text-[13.5px] motion-tint desktop:min-h-0 desktop:py-1.5 ${
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

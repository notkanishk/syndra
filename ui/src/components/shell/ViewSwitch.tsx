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
            className={`rounded-pill px-4 py-1.5 text-[13.5px] transition-colors duration-150 ${
              active
                ? "bg-accent font-semibold text-accent-ink"
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

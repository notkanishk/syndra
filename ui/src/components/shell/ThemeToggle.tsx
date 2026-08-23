"use client";

import { useTheme } from "@/lib/theme";

/**
 * Dark is home; light is daylight. The toggle is a quiet neighbour of the
 * account chip — it changes how the room looks, never what is in it.
 */
export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const next = theme === "dark" ? "light" : "dark";

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={`Switch to ${next} theme`}
      // A 32px ring inside a 44px box until desktop. The ring is the drawing;
      // the box is what a thumb on a tablet has to hit.
      className="flex h-11 w-11 items-center justify-center text-muted motion-tint hover:text-ink desktop:h-8 desktop:w-8"
    >
      <span
        aria-hidden
        className="flex h-8 w-8 items-center justify-center rounded-pill border border-line-strong"
      >
        {theme === "dark" ? <SunIcon /> : <MoonIcon />}
      </span>
    </button>
  );
}

/* Lucide, stroke 1.5 — the icon set this system uses wherever a geometric
   primitive would not read faster. */

function SunIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      aria-hidden
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
    </svg>
  );
}

"use client";

import { useEffect, useId, useRef, useState } from "react";

import { PILL } from "@/components/ui/Button";

const PILL_BOX = `inline-flex items-center justify-center rounded-pill font-semibold motion-tint ${PILL.md}`;

/**
 * A page's filters, behind one button.
 *
 * Six controls laid out along a page header is not a filter bar, it is a form
 * nobody asked for: on Drift they wrapped to five stacked full-width rows and
 * pushed the queue itself below the fold. Put together in a panel they read as
 * one thing — a set of narrowings, with labels, in a column — and the page gets
 * its header back.
 *
 * The button carries the count of what is active, so the panel never has to be
 * opened to answer "why am I seeing so few rows". That question is the one a
 * hidden filter creates, and the count is the whole of the answer.
 *
 * Native `<details>` was the tempting shortcut. It positions in flow, so the
 * panel would push the page down as it opens; and it cannot be dismissed by
 * Escape or by clicking away without the same handlers written here anyway.
 */
export function FilterPanel({
  activeCount,
  onClear,
  children,
}: {
  /** How many filters are narrowing the list right now. */
  activeCount: number;
  onClear: () => void;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    function onDocument(event: MouseEvent) {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDocument);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocument);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((value) => !value)}
        className={`${PILL_BOX} border bg-transparent hover:text-ink ${
          activeCount > 0 ? "border-accent-line text-ink" : "border-line-strong text-muted"
        }`}
      >
        Filters
        {activeCount > 0 && (
          <span className="ml-2 rounded-pill bg-accent-dense px-2 py-0.5 text-[12.5px] font-bold text-accent-ink">
            {activeCount}
          </span>
        )}
      </button>

      {open && (
        <div
          id={id}
          // Right-aligned: the button sits at the end of its row on wide
          // screens, and a panel hanging off the left of it would run past the
          // window edge. z-40 clears the page and stays under a modal.
          className="settle-in absolute right-0 top-[calc(100%+8px)] z-40 w-[min(22rem,calc(100vw-2rem))] rounded-panel border border-line-strong bg-surface-2 p-4 shadow-popover"
        >
          <div className="flex flex-col gap-3.5">{children}</div>
          <div className="mt-4 flex items-center justify-between border-t border-line pt-3.5">
            <button
              type="button"
              onClick={onClear}
              disabled={activeCount === 0}
              className="text-[13.5px] font-semibold text-muted motion-tint hover:text-ink disabled:cursor-not-allowed disabled:text-faint"
            >
              Clear all
            </button>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className={`${PILL_BOX} border border-line-strong bg-transparent text-ink`}
            >
              Done
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * One labelled row inside the panel. The label is a real `<label>` when the
 * control is a single field, and a group label when it is a set of pills —
 * which is why it takes the rendered control rather than rendering one itself.
 */
export function FilterField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="type-label text-[12.5px] text-label">{label}</span>
      {children}
    </div>
  );
}

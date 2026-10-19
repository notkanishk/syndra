"use client";

import { useEffect, useId, useRef, useState } from "react";

import { AnchoredPanel } from "@/components/ui/Anchored";
import { Button, PILL } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useIsPhone } from "@/lib/useViewport";

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
  resultCount,
  noun,
  children,
}: {
  /** How many filters are narrowing the list right now. */
  activeCount: number;
  onClear: () => void;
  /** The list's live count after these filters, so the phone sheet's button names the result. */
  resultCount?: number;
  noun?: [string, string];
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const phone = useIsPhone();
  const ref = useRef<HTMLDivElement>(null);
  const panel = useRef<HTMLDivElement | null>(null);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    function onDocument(event: MouseEvent) {
      // The panel is under <body>, outside the button's wrapper.
      const target = event.target as Node;
      if (ref.current?.contains(target) || panel.current?.contains(target))
        return;
      setOpen(false);
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
          activeCount > 0
            ? "border-accent-line text-ink"
            : "border-line-strong text-muted"
        }`}
      >
        Filters
        {activeCount > 0 && (
          <span className="ml-2 rounded-pill bg-accent-dense px-2 py-0.5 text-[12.5px] font-bold text-accent-ink">
            {activeCount}
          </span>
        )}
      </button>

      {/* Below tablet (87c): a sheet ending in one accent button that names
          the result, so the count answers "why so few" before it closes. */}
      {phone ? (
        <Modal
          open={open}
          onClose={() => setOpen(false)}
          size="sm"
          labelledBy={`${id}-title`}
        >
          <ModalHeader title="Filters" titleId={`${id}-title`} />
          <div id={id} className="flex flex-col gap-3.5 px-6">
            {children}
          </div>
          <ModalFooter>
            <Button variant="accent" onClick={() => setOpen(false)}>
              {resultCount !== undefined && noun
                ? `Show ${resultCount} ${resultCount === 1 ? noun[0] : noun[1]}`
                : "Done"}
            </Button>
            <Button
              variant="ghost"
              onClick={onClear}
              disabled={activeCount === 0}
            >
              Clear all
            </Button>
          </ModalFooter>
        </Modal>
      ) : (
        <AnchoredPanel
          anchor={ref}
          open={open}
          id={id}
          panelRef={panel}
          // Right-aligned: the button sits at the end of its row on wide
          // screens, and a panel hanging off the left of it would run past the
          // window edge.
          align="end"
          width={352}
          expectHeight={420}
          className="popover-in flex max-h-[min(72vh,34rem)] flex-col rounded-panel border border-line-strong bg-surface-2 p-4 shadow-popover"
        >
          {/* The fields scroll, the footer does not. Six of them run past the
              bottom of a laptop window, and Clear and Done are the two
              controls somebody needs once they have finished choosing. */}
          <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto">
            {children}
          </div>
          <div className="mt-4 flex shrink-0 items-center justify-between border-t border-line pt-3.5">
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
        </AnchoredPanel>
      )}
    </div>
  );
}

/**
 * One labelled row inside the panel. The label is a real `<label>` when the
 * control is a single field, and a group label when it is a set of pills —
 * which is why it takes the rendered control rather than rendering one itself.
 */
export function FilterField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="type-label text-[12.5px] text-label">{label}</span>
      {children}
    </div>
  );
}

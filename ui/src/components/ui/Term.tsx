"use client";

import { useEffect, useId, useRef, useState } from "react";

import { GLOSSARY, type TermName } from "@/lib/glossary";

/**
 * A word that carries its own definition.
 *
 * The product used to explain its vocabulary inline — "Zitadel (the service
 * everyone signs in through)" — which told a reader who runs the thing
 * something they already knew, in the middle of the sentence they were trying
 * to read. This puts the term back in the sentence and the definition one
 * hover, tap, or Tab away.
 *
 * It is a `<button>`, not a `<span>` with a mouse handler, and that decision
 * is the whole accessibility story: a button is in the tab order for free,
 * fires on Enter and Space for free, and is announced as interactive for free.
 * Hover alone would have hidden every definition from anybody who does not
 * use a mouse — which on this deployment includes anybody on the workshop
 * tablet.
 *
 * The definition is rendered ONCE, always in the DOM, and referenced by
 * `aria-describedby`, so a screen reader reads the term and its meaning
 * together whether or not the popover is open. When it is shut, it is hidden
 * the sr-only way (clipped, not `display: none`) — `display: none` content is
 * skipped by `aria-describedby` in several screen readers, which would have
 * made the marked-up word *less* informative than the plain one.
 *
 * Opening is sticky (click, and it stays until dismissed) because a definition
 * you cannot keep on screen is one you have to re-open to re-read, and on a
 * touch screen there is no hover to fall back on.
 */
export function Term({ name, children }: { name: TermName; children?: React.ReactNode }) {
  const entry = GLOSSARY[name];
  const id = useId();
  const [open, setOpen] = useState(false);
  const [hovered, setHovered] = useState(false);
  const ref = useRef<HTMLSpanElement | null>(null);

  const shown = open || hovered;

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
    <span
      ref={ref}
      className="relative inline-block"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <button
        type="button"
        aria-describedby={id}
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        onFocus={() => setHovered(true)}
        onBlur={() => setHovered(false)}
        className="cursor-help underline decoration-dotted decoration-from-font underline-offset-[3px] motion-tint hover:text-ink"
      >
        {children ?? entry.title}
      </button>

      {/* One node, two states. Shut, it is clipped rather than removed, so
          `aria-describedby` still resolves to it. */}
      <span
        id={id}
        role="note"
        className={
          shown
            ? "settle-in absolute left-0 top-[calc(100%+7px)] z-30 w-[min(19rem,calc(100vw-2rem))] rounded-panel border border-line-strong bg-surface-2 px-3.5 py-3 text-left shadow-popover"
            : "sr-only"
        }
      >
        <span className="block type-label">{entry.title}</span>
        <span className="mt-1 block text-[13.5px] font-normal leading-[1.55] text-muted">
          {entry.definition}
        </span>
      </span>
    </span>
  );
}

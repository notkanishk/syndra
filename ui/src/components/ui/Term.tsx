"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { anchoredStyle, useAnchoredBox } from "@/components/ui/Anchored";
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
 *
 * The OPEN popover is rendered into `document.body` rather than beside the
 * button. Every card in this product is `overflow-hidden` — that is what keeps
 * its rounded corners clipping its rows — so a popover positioned inside one is
 * cut off at the card's edge, which is exactly what happened on Home: the
 * definition opened downwards and the bottom two thirds of it were sliced away
 * by the card it belonged to. Portalling escapes the clip without asking every
 * card in the product to stop clipping. The node keeps its id, so
 * `aria-describedby` still resolves across the document.
 *
 * NOTE FOR TESTS: the definition is a sibling of the button and inside the same
 * paragraph, so it lands in that paragraph's `textContent` even while clipped.
 * A sentence containing a term is therefore never contiguous — `getByText(/the
 * whole sentence/)` will not match. Assert either side of the term, or query
 * the term itself. The definition is deliberately NOT `aria-hidden`: it is what
 * `aria-describedby` resolves to, and hiding it would leave the marked-up word
 * with no meaning attached for exactly the readers who most need one.
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

  // document.body exists only after mount; a portal on the server render
  // would be a hydration mismatch.
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  // Where to draw it, measured from the trigger (shared with every popover).
  const box = useAnchoredBox(ref, shown, 304, 180);

  const panel = (
    <span
      id={id}
      role="note"
      style={anchoredStyle(box)}
      className="settle-in rounded-panel border border-line-strong bg-surface-2 px-3.5 py-3 text-left shadow-popover"
    >
      <span className="block type-label">{entry.title}</span>
      <span className="mt-1 block text-[13.5px] font-normal leading-[1.55] text-muted">
        {entry.definition}
      </span>
    </span>
  );

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

      {/* One node, two states. Shut, it stays in the DOM and is clipped
          rather than removed, so `aria-describedby` still resolves to it —
          `display: none` content is skipped by several screen readers. Open,
          it is the same content portalled out of the card that would clip it.
          Only one of the two carries the id at a time. */}
      {shown ? null : (
        <span id={id} role="note" className="sr-only">
          <span className="block type-label">{entry.title}</span>
          <span className="mt-1 block text-[13.5px] font-normal leading-[1.55] text-muted">
            {entry.definition}
          </span>
        </span>
      )}
      {shown && mounted ? createPortal(panel, document.body) : null}
    </span>
  );
}

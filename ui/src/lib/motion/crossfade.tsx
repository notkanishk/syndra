"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

import { stillTint, TINT_MS } from "./still";

/**
 * One seat whose words change in place (Motion v2 S4): the old layer fades out
 * and the new one fades in on tint, both in the same grid cell, so the seat is
 * as big as the taller of the two and nothing around it shifts.
 *
 * `id` is what counts as a change of word. New children under the same id just
 * replace the old ones. Reduced motion keeps the crossfade: it is tint, and tint
 * survives (A-24 Q15).
 */
export function Crossfade({
  id,
  children,
  className = "",
  inline = false,
}: {
  id: string;
  children: ReactNode;
  className?: string;
  /** Words inside a sentence (Home's headline): spans, and the seat sits in the line. */
  inline?: boolean;
}) {
  const Layer = inline ? "span" : "div";
  const last = useRef({ id, children });
  const [leaving, setLeaving] = useState<ReactNode>(null);
  const [fading, setFading] = useState(false);

  if (last.current.id !== id) {
    const was = last.current.children;
    last.current = { id, children };
    if (!stillTint()) {
      setLeaving(was);
      setFading(true);
    }
  } else {
    last.current = { id, children };
  }

  useEffect(() => {
    if (!fading) return;
    const timer = setTimeout(() => {
      setLeaving(null);
      setFading(false);
    }, TINT_MS);
    return () => clearTimeout(timer);
  }, [fading, id]);

  return (
    <Layer className={`${inline ? "inline-grid" : "grid"} [&>*]:col-start-1 [&>*]:row-start-1 ${className}`}>
      {fading && (
        <Layer key="leaving" className="tint-out min-w-0" aria-hidden inert>
          {leaving}
        </Layer>
      )}
      <Layer key={id} className={`min-w-0 ${fading ? "tint-in" : ""}`}>
        {children}
      </Layer>
    </Layer>
  );
}

/**
 * A figure whose digits change in place (A-24 §11.2): only what changed,
 * changes. 47 becoming 46 crossfades its last digit and the 4 holds still.
 * Each digit is its own seat, counted from the right so units stay over units;
 * a digit that appears (9 to 10) is simply there. Nothing ticks or rolls.
 *
 * The live digits are plain inline text, so the figure reads and copies as one
 * number (a grid seat per digit would be a block per digit to the clipboard
 * and to a screen reader). Only the digit leaving is laid over its seat, out of
 * the flow, hidden from assistive technology and from selection. Every seat is
 * one width, because every figure is tabular (the body rule in globals.css).
 */
export function CrossfadeDigits({ text }: { text: string }) {
  const chars = Array.from(text);
  return (
    <span data-figure>
      {chars.map((char, index) => (
        <Digit key={chars.length - index} char={char} />
      ))}
    </span>
  );
}

function Digit({ char }: { char: string }) {
  const last = useRef(char);
  const [leaving, setLeaving] = useState<string | null>(null);
  if (last.current !== char) {
    const was = last.current;
    last.current = char;
    if (!stillTint()) setLeaving(was);
  }

  useEffect(() => {
    if (leaving === null) return;
    const timer = setTimeout(() => setLeaving(null), TINT_MS);
    return () => clearTimeout(timer);
  }, [leaving, char]);

  return (
    <span className="relative">
      {leaving !== null && (
        <span aria-hidden className="tint-out pointer-events-none absolute inset-0 select-none">
          {leaving}
        </span>
      )}
      <span key={char} className={leaving !== null ? "tint-in" : undefined}>
        {char}
      </span>
    </span>
  );
}

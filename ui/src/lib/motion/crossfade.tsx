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
 * Digits are matched from the right, so units stay over units; a digit that
 * appears (9 to 10) tints in where nothing was. Nothing ticks or rolls.
 *
 * At rest the figure is its plain text, one word to a screen reader and to the
 * clipboard. Only for the one tint while a digit fades is it split: the new
 * digits inline, the changed ones tinting in, and the old text laid over them
 * from the right, out of the flow, hidden from assistive technology and from
 * selection, with only its changed digits showing as they fade. Seats line up
 * because every figure is tabular (`[data-figure]` in globals.css).
 */
export function CrossfadeDigits({ text }: { text: string }) {
  const last = useRef(text);
  const [was, setWas] = useState<string | null>(null);
  if (last.current !== text) {
    const previous = last.current;
    last.current = text;
    if (!stillTint()) setWas(previous);
  }

  useEffect(() => {
    if (was === null) return;
    const timer = setTimeout(() => setWas(null), TINT_MS);
    return () => clearTimeout(timer);
  }, [was, text]);

  if (was === null) return <span data-figure className="relative">{text}</span>;
  const now = Array.from(text);
  const before = Array.from(was);
  // The digit `r` places from the right, or undefined.
  const at = (chars: string[], r: number) => chars[chars.length - 1 - r];
  return (
    <span data-figure className="relative">
      {now.map((char, i) => {
        const r = now.length - 1 - i;
        return (
          <span key={r} className={at(before, r) === char ? undefined : "tint-in"}>
            {char}
          </span>
        );
      })}
      <span aria-hidden className="pointer-events-none absolute right-0 top-0 select-none whitespace-nowrap">
        {before.map((char, i) => {
          const r = before.length - 1 - i;
          return (
            <span key={r} className={at(now, r) === char ? "invisible" : "tint-out"}>
              {char}
            </span>
          );
        })}
      </span>
    </span>
  );
}

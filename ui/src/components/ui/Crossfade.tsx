"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

import { stillMotion, TINT_MS } from "@/lib/motion";

/**
 * One seat whose words change in place (Motion v2 S4): the old layer fades out
 * and the new one fades in on tint, both in the same grid cell, so the seat is
 * as big as the taller of the two and nothing around it shifts.
 *
 * `id` is what counts as a change of word. New children under the same id just
 * replace the old ones. Under reduced motion the words swap at once.
 */
export function Crossfade({
  id,
  children,
  className = "",
}: {
  id: string;
  children: ReactNode;
  className?: string;
}) {
  const last = useRef({ id, children });
  const [leaving, setLeaving] = useState<ReactNode>(null);
  const [fading, setFading] = useState(false);

  if (last.current.id !== id) {
    const was = last.current.children;
    last.current = { id, children };
    if (!stillMotion()) {
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
    <div className={`grid [&>*]:col-start-1 [&>*]:row-start-1 ${className}`}>
      {fading && (
        <div key="leaving" className="tint-out min-w-0" aria-hidden inert>
          {leaving}
        </div>
      )}
      <div key={id} className={`min-w-0 ${fading ? "tint-in" : ""}`}>
        {children}
      </div>
    </div>
  );
}

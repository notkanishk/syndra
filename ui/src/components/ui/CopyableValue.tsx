"use client";

import { useEffect, useRef, useState } from "react";

/**
 * A value somebody retypes into another application.
 *
 * Mono, on its own ground, copyable — the same treatment `CommandBlock` gives a
 * terminal line, for the same reason: this is not prose to be read, it is a
 * string to be transported, and the failure mode is a transcription error the
 * member then spends twenty minutes hunting.
 *
 * Separate from `CommandBlock` because that component is about a command and
 * the steps around it. This is one value on one line, and the member surfaces
 * are full of them: an account name, a share path, a host.
 */
export function CopyableValue({
  value,
  label,
  className = "",
}: {
  value: string;
  /** What the value IS, for a screen reader. Never rendered as a placeholder. */
  label: string;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 1800);
    } catch {
      // A clipboard a browser refuses is not an error worth a banner: the value
      // is on screen and selectable, which is the fallback that always works.
    }
  }

  return (
    <div
      className={`flex items-center gap-3 rounded-inner border border-line bg-surface-0 px-3.5 py-2.5 ${className}`}
    >
      <code className="min-w-0 flex-1 truncate font-mono text-[14px] text-ink">{value}</code>
      <button
        type="button"
        onClick={copy}
        className="shrink-0 rounded-pill px-2.5 py-1 text-[12.5px] font-semibold text-muted motion-tint hover:bg-[var(--hover)] hover:text-ink"
      >
        {copied ? "Copied" : "Copy"}
      </button>
      {/* Announced rather than only coloured: the label change is the whole
          feedback, and a sighted user gets it from the button's own text. */}
      <span aria-live="polite" className="sr-only">
        {copied ? `${label} copied` : ""}
      </span>
    </div>
  );
}

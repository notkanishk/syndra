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
 *
 * **It knows whether it can copy before it is tapped.** `navigator.clipboard`
 * is undefined on an insecure origin, and this deployment is reached over http
 * on a LAN — so on the network most members actually use, a Copy button is an
 * affordance that does nothing. Rather than fail silently on the tap, the row
 * offers `Select` instead and says why. The value is fine; the browser is the
 * limitation, and the copy says so in that order.
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
  const [selected, setSelected] = useState(false);
  // "Your account name" mid-sentence is "your account name"; "TrueNAS host" stays.
  const object = label.replace(/^[A-Z](?=[a-z])/, (c) => c.toLowerCase());
  // Resolved after mount. Rendering the answer on the server would mean a
  // hydration mismatch on every insecure origin — and getting it wrong in
  // that direction offers a control that cannot work.
  const [canCopy, setCanCopy] = useState(true);
  const valueRef = useRef<HTMLElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setCanCopy(typeof navigator !== "undefined" && Boolean(navigator.clipboard));
  }, []);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  /** 900ms, and then the row is a row again. No toast: on a phone a toast
      covers the value that was just copied. */
  function confirmFor(set: (on: boolean) => void) {
    set(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => set(false), 900);
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      confirmFor(setCopied);
    } catch {
      // A clipboard the browser refuses at the moment of the tap: fall back to
      // the same selection the insecure-origin path uses, rather than leaving
      // the operator with a control that appeared to do nothing.
      setCanCopy(false);
      select();
    }
  }

  /** Puts the value under the platform's own copy gesture. */
  function select() {
    const node = valueRef.current;
    if (!node) return;
    const selection = window.getSelection?.();
    if (!selection) return;
    selection.removeAllRanges();
    const range = document.createRange();
    range.selectNodeContents(node);
    selection.addRange(range);
    confirmFor(setSelected);
  }

  return (
    <div
      className={`flex min-h-[52px] items-center gap-3 rounded-inner border px-3.5 py-2.5 motion-tint ${
        copied || selected ? "border-line bg-healthy/[.07]" : "border-line bg-surface-0"
      } ${className}`}
    >
      {/* Wraps rather than truncates, at any width. An operator reading a path
          aloud to somebody standing at a machine needs all of it, and half a
          share path is worse than none because it looks complete. */}
      <code ref={valueRef} className="min-w-0 flex-1 break-all font-mono text-[14px] text-ink">
        {value}
      </code>

      <button
        type="button"
        onClick={canCopy ? copy : select}
        // Six rows on one page are six buttons, and "Copy" six times is one
        // name for six things. The name says what it copies.
        aria-label={`${canCopy ? (copied ? "Copied" : "Copy") : selected ? "Selected" : "Select"} ${object}`}
        className={`min-h-[44px] shrink-0 rounded-pill px-2.5 text-[12.5px] font-semibold motion-tint ${
          copied || selected ? "text-healthy" : "text-muted hover:bg-[var(--hover)] hover:text-ink"
        }`}
      >
        {canCopy ? (copied ? "Copied" : "Copy") : selected ? "Selected" : "Select"}
      </button>

      {/* Announced rather than only coloured: the label change is the whole
          feedback, and a sighted user gets it from the button's own text. */}
      <span aria-live="polite" className="sr-only">
        {copied ? `${label} copied` : selected ? `${label} is selected — hold to copy` : ""}
      </span>
    </div>
  );
}

/**
 * The sentence that goes under a copy row when the browser cannot copy.
 *
 * Separate from the row so a screen showing six of them says it once. It is
 * not an error: nothing is wrong with the value, and the member is one long
 * press away from having it.
 */
export function ClipboardUnavailableNote({ className = "" }: { className?: string }) {
  const [canCopy, setCanCopy] = useState(true);
  useEffect(() => {
    setCanCopy(typeof navigator !== "undefined" && Boolean(navigator.clipboard));
  }, []);

  if (canCopy) return null;
  return (
    <p className={`text-[12.5px] leading-[1.5] text-faint ${className}`}>
      Your browser can&apos;t copy on this connection. Tap a value to select it, then hold to
      copy.
    </p>
  );
}

"use client";

import { useEffect, useRef, useState } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { CopiedSeat, useConfirmFor } from "@/components/ui/CopiedSeat";
import { TINT_MS, useExitPhase } from "@/lib/motion";

/**
 * Whether this page can put text on the clipboard: the async Clipboard API (secure origins only),
 * or `document.execCommand("copy")`, which is deprecated but is the only way on the plain-http LAN
 * this is reached over, and still works in every browser.
 */
const clipboardWorks = () =>
  typeof navigator !== "undefined" &&
  (Boolean(navigator.clipboard) || (typeof document !== "undefined" && Boolean(document.queryCommandSupported?.("copy"))));

/**
 * The plain-http copy: a hidden read-only textarea holds the text, `execCommand("copy")` copies its
 * selection, and the reader's own selection and focus are put back (the standard fallback; the
 * page must not lose the selection or the focus place to a copy). True only if the browser says
 * it copied.
 */
function copyViaTextarea(value: string): boolean {
  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const selection = window.getSelection?.();
  const ranges = selection ? Array.from({ length: selection.rangeCount }, (_, i) => selection.getRangeAt(i).cloneRange()) : [];
  const area = document.createElement("textarea");
  area.value = value;
  area.readOnly = true; // no on-screen keyboard on a phone
  area.setAttribute("aria-label", "Text being copied"); // it is briefly in the page; a reader names it
  area.style.cssText = "position:fixed;top:0;left:0;opacity:0;pointer-events:none";
  document.body.appendChild(area);
  try {
    area.focus({ preventScroll: true }); // selecting needs focus in some browsers (Safari)
    area.select();
    area.setSelectionRange(0, value.length);
    return document.execCommand("copy");
  } finally {
    area.remove();
    // Focus first: giving an element focus moves the selection, so the reader's ranges go back after it.
    active?.focus({ preventScroll: true });
    selection?.removeAllRanges();
    for (const range of ranges) selection?.addRange(range);
  }
}

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
  inline = false,
}: {
  value: string;
  /** What the value IS, for a screen reader. Never rendered as a placeholder. */
  label: string;
  className?: string;
  /** The value alone, faint mono, in running text: the whole value is the copy control. */
  inline?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [selected, setSelected] = useState(false);
  const [failed, setFailed] = useState(false);
  // "Your account name" mid-sentence is "your account name"; "TrueNAS host" stays.
  const object = label.replace(/^[A-Z](?=[a-z])/, (c) => c.toLowerCase());
  // Resolved after mount. Rendering the answer on the server would mean a
  // hydration mismatch on every insecure origin — and getting it wrong in
  // that direction offers a control that cannot work.
  const [canCopy, setCanCopy] = useState(true);
  const valueRef = useRef<HTMLElement>(null);
  // 900ms, and then the row is a row again. No toast: on a phone a toast
  // covers the value that was just copied.
  const confirmFor = useConfirmFor();
  // Inline, "Copied" is in the text only while it is showing: a value in a
  // sentence is selected and copied by hand too, and must never carry the word.
  const word = useExitPhase(copied, TINT_MS);

  useEffect(() => {
    setCanCopy(clipboardWorks());
  }, []);

  async function copy() {
    try {
      if (navigator.clipboard) await navigator.clipboard.writeText(value);
      else if (!copyViaTextarea(value)) throw new Error("copy refused"); // plain http: no Clipboard API
      setFailed(false);
      confirmFor(setCopied);
    } catch {
      // A clipboard the browser refuses at the moment of the tap: fall back to
      // the same selection the insecure-origin path uses, rather than leaving
      // the operator with a control that appeared to do nothing.
      setCanCopy(false);
      setFailed(true);
      select();
    }
  }

  /** Selects the value's text; false when there is nothing to select it in. */
  function highlight() {
    const node = valueRef.current;
    const selection = window.getSelection?.();
    if (!node || !selection) return false;
    selection.removeAllRanges();
    const range = document.createRange();
    range.selectNodeContents(node);
    selection.addRange(range);
    return true;
  }

  /** Puts the value under the platform's own copy gesture. */
  function select() {
    if (highlight()) confirmFor(setSelected);
  }

  if (inline) {
    return (
      <button
        type="button"
        data-skip-initial-focus
        onClick={(event) => {
          // Names sit inside links and rows; copying an id must not also open them.
          event.preventDefault();
          event.stopPropagation();
          return canCopy ? copy() : select();
        }}
        aria-label={`${canCopy ? "Copy" : "Select"} ${object}`}
        className={`type-mono text-faint control press-scale hover:text-ink ${copied || selected ? "text-healthy" : ""} ${className}`}
      >
        {/* The value and "Copied" share one seat, so the line never changes
            width (S18). A selection keeps the value on screen: it is what is
            selected. */}
        <span className="inline-grid items-center justify-items-start [&>*]:col-start-1 [&>*]:row-start-1">
          <span ref={valueRef} className={`motion-tint ${copied ? "opacity-0" : "opacity-100"}`}>
            {value}
          </span>
          {word.rendered && (
            <span
              aria-hidden
              className={`isolate inline-flex select-none items-center gap-2 ${word.leaving ? "tint-out" : "tint-in"}`}
            >
              <StatusDot tone="healthy" glow={copied} />
              Copied
            </span>
          )}
        </span>
      </button>
    );
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
      <code ref={valueRef} className="min-w-0 flex-1 break-all font-mono text-body text-ink">
        {value}
      </code>

      <button
        type="button"
        onClick={canCopy ? copy : select}
        // Six rows on one page are six buttons, and "Copy" six times is one
        // name for six things. The name says what it copies.
        // `data-inline`: the control belongs to the value it copies, not to a row's seat (D-1 §4).
        data-inline
        aria-label={`${canCopy ? (copied ? "Copied" : "Copy") : selected ? "Selected" : "Select"} ${object}`}
        className={`inline-flex min-h-[var(--tap)] shrink-0 items-center gap-2 rounded-pill px-2.5 text-fine font-semibold control ${
          copied || selected ? "text-healthy" : "text-muted hover:bg-[var(--hover)] hover:text-ink"
        }`}
      >
        {/* Only a copy blooms: it went somewhere. A selection is still the reader's
            to finish. */}
        <CopiedSeat
          done={copied || selected}
          idle={canCopy ? "Copy" : "Select"}
          word={canCopy ? "Copied" : "Selected"}
          dot={canCopy}
        />
      </button>

      {/* Announced rather than only coloured: the label change is the whole
          feedback, and a sighted user gets it from the button's own text. */}
      <span aria-live="polite" className="sr-only">
        {copied
          ? `${label} copied`
          : failed
            ? `Couldn't copy ${object}: select it and copy`
            : selected ? `${label} is selected. Hold to copy` : ""}
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
    setCanCopy(clipboardWorks());
  }, []);

  if (canCopy) return null;
  return (
    <p className={`text-fine leading-[1.5] text-faint ${className}`}>
      Your browser can&apos;t copy on this connection. Tap a value to select it, then hold to
      copy.
    </p>
  );
}

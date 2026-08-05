"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";

/**
 * A command the operator runs in their own terminal, and the steps around it.
 *
 * Every one of these could have been a button. None of them is, and that is the
 * point. Rotating a signing key, wiping seeded fixtures, restarting a container
 * — each has a failure mode where the console reports success and the system is
 * in the half-state nobody wanted (Zitadel took the new key, the backend is
 * still verifying against the old one). The operator running it themselves has
 * the exit code, the stderr and the ability to stop halfway; a spinner in a web
 * page has none of that.
 *
 * So the screen's job is to remove the part that actually goes wrong, which is
 * remembering the command and what to do after it. Copy the line, follow the
 * numbered steps, come back.
 *
 * `steps` render as an ordered list beneath the command. Use them for anything
 * that must happen after it — an env swap, a restart — because a command whose
 * follow-up lives in a README is a command that gets run halfway.
 */
export function CommandBlock({
  command,
  caption,
  steps,
  tone = "neutral",
  label,
}: {
  command: string;
  /** One line on what running this does. Rendered above the command. */
  caption?: string;
  /** What must happen after it, in order. */
  steps?: React.ReactNode[];
  /**
   * `warn` for a command that changes production state. `onWarn` for the same
   * block sitting on top of an amber banner, where the page's own surface
   * colours would vanish into the background.
   */
  tone?: "neutral" | "warn" | "onWarn";
  /** Overrides the default accessible name of the copy button. */
  label?: string;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // Without this, copying and then navigating away sets state on an unmounted
  // component — and in a page that polls, remounts are routine.
  useEffect(() => () => clearTimeout(timer.current), []);

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access is denied on non-secure origins, which is exactly
      // where this app runs on a LAN IP. The command stays selectable text,
      // so the fallback is the operator highlighting it — no error to show.
    }
  }, [command]);

  const onWarn = tone === "onWarn";

  return (
    <div
      className={
        onWarn
          ? "rounded-block bg-warn-ink/[.07] px-4 py-3.5"
          : tone === "warn"
            ? "warn-note px-4 py-3.5"
            : "rounded-block bg-tint-1 px-4 py-3.5"
      }
    >
      {caption && (
        <p
          className={`mb-2.5 max-w-[80ch] text-[13.5px] leading-[1.55] ${
            onWarn ? "text-warn-ink/[.78]" : "text-muted"
          }`}
        >
          {caption}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <code
          className={`type-mono min-w-0 flex-1 overflow-x-auto rounded-inner px-3 py-2 ${
            onWarn ? "bg-warn-ink/10 text-warn-ink" : "bg-tint-2 text-ink"
          }`}
        >
          {command}
        </code>
        <button
          type="button"
          onClick={copy}
          aria-label={label ?? `Copy command: ${command}`}
          className={`shrink-0 rounded-pill px-3.5 py-1.5 text-[13px] font-semibold motion-tint ${
            onWarn
              ? "border border-warn-ink/30 text-warn-ink hover:bg-warn-ink/10"
              : "border border-line-strong text-ink hover:bg-[var(--hover)]"
          }`}
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>

      {/* aria-live so the confirmation reaches a screen reader; the button
          label alone changes silently. */}
      <span aria-live="polite" className="sr-only">
        {copied ? "Command copied to clipboard" : ""}
      </span>

      {steps && steps.length > 0 && (
        <ol className="mt-3 flex list-none flex-col gap-1.5">
          {steps.map((step, index) => (
            <li
              key={index}
              className={`flex gap-2.5 text-[13.5px] leading-[1.55] ${
                onWarn ? "text-warn-ink/[.78]" : "text-muted"
              }`}
            >
              <span
                className={`type-mono shrink-0 pt-px ${onWarn ? "text-warn-ink/60" : "text-faint"}`}
              >
                {index + 1}.
              </span>
              <span className="max-w-[76ch]">{step}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

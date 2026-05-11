"use client";

import { useState } from "react";

interface CopyButtonProps {
  text: string;
  label?: string;
  className?: string;
}

/**
 * Click-to-copy button with confirmation. Used by the Token Simulator and
 * other code-block surfaces. Falls back silently if clipboard API is denied.
 */
export function CopyButton({ text, label = "Copy", className = "" }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      // Browser may deny clipboard access — leave UI unchanged.
    }
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={`${label} to clipboard`}
      className={`inline-flex items-center gap-1.5 rounded-md border border-outline-variant px-2 py-1 text-xs text-on-surface-variant transition-colors hover:text-on-surface hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${className}`}
    >
      <svg
        viewBox="0 0 24 24"
        width="12"
        height="12"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
      </svg>
      {copied ? "Copied!" : label}
    </button>
  );
}

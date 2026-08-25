"use client";

import { useEffect, useState } from "react";

/**
 * Whether this browser can copy at all.
 *
 * `navigator.clipboard` is undefined on an insecure origin, and this product
 * is reached over http on a LAN — so the answer is routinely no, and it is
 * knowable before anybody taps anything. Three surfaces need it (a value, a
 * command, a token payload), which is two more than should each carry their
 * own copy of the question.
 *
 * Resolved after mount rather than during render: answering on the server
 * would mean a hydration mismatch on every insecure origin, and the direction
 * that guesses wrong is the one that offers a control which cannot work.
 */
export function useClipboardAvailable(): boolean {
  const [available, setAvailable] = useState(true);
  useEffect(() => {
    setAvailable(typeof navigator !== "undefined" && Boolean(navigator.clipboard));
  }, []);
  return available;
}

/** Puts a node's text under the platform's own copy gesture. */
export function selectContents(node: HTMLElement | null): boolean {
  const selection = typeof window !== "undefined" ? window.getSelection?.() : null;
  if (!node || !selection) return false;
  selection.removeAllRanges();
  const range = document.createRange();
  range.selectNodeContents(node);
  selection.addRange(range);
  return true;
}

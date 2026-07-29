"use client";

import { useEffect, useRef } from "react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  /** Aria-labelledby target. Recommended: heading id inside the modal. */
  labelledBy?: string;
  /** Aria-describedby target. Recommended: description id inside the modal. */
  describedBy?: string;
  /** Disable Esc and click-outside dismiss while a mutation is in flight. */
  busy?: boolean;
  /** Modal sizing — sm: 24rem, md: 32rem, lg: 48rem. Default md. */
  size?: "sm" | "md" | "lg";
  /** Children rendered inside the focus-trapped panel. */
  children: React.ReactNode;
}

const SIZE_CLASS: Record<NonNullable<ModalProps["size"]>, string> = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-3xl",
};

const FOCUSABLE_SELECTOR =
  "button, [href], input, select, textarea, [tabindex]:not([tabindex='-1'])";

/**
 * Shared dialog behavior for Modal and Drawer: focus the first focusable
 * element on open, trap Tab inside the panel, Esc-to-close (unless busy),
 * restore focus to the previously focused element on close.
 */
export function useDialogFocusTrap(
  panelRef: React.RefObject<HTMLDivElement | null>,
  open: boolean,
  busy: boolean,
  onClose: () => void,
) {
  useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;

    panelRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)[0]?.focus();

    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape" && !busy) {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab") return;
      if (!panelRef.current) return;
      const list = panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
      if (list.length === 0) return;
      const first = list[0];
      const last = list[list.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("keydown", handleKey);
      previouslyFocused?.focus();
    };
  }, [panelRef, open, busy, onClose]);
}

/**
 * Generic modal primitive — focus trap, aria-modal, Esc + click-outside
 * dismiss, glass-card panel. Use this directly for confirmation flows and
 * compose `<ConfirmModal/>` (destructive footer) on top of it. Drawer mirrors
 * the same a11y model with right-side slide-in geometry.
 */
export function Modal({
  open,
  onClose,
  labelledBy,
  describedBy,
  busy = false,
  size = "md",
  children,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  useDialogFocusTrap(panelRef, open, busy, onClose);

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4 backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      <div
        ref={panelRef}
        className={`glass-card w-full ${SIZE_CLASS[size]} p-6 animate-fade-in-up`}
      >
        {children}
      </div>
    </div>
  );
}

"use client";

import { useEffect, useRef } from "react";

/**
 * The dialog primitive. Destructive actions ALWAYS open one of these — never a
 * bare confirm(), never an inline undo-only toast.
 *
 * surface-2, 22px radius, the dialog shadow, focus-trapped, Esc to dismiss
 * unless a mutation is in flight.
 */

interface ModalProps {
  open: boolean;
  onClose: () => void;
  labelledBy?: string;
  describedBy?: string;
  /** Disable Esc and click-outside dismiss while a mutation is in flight. */
  busy?: boolean;
  size?: "sm" | "md" | "lg";
  children: React.ReactNode;
}

const SIZE_CLASS: Record<NonNullable<ModalProps["size"]>, string> = {
  sm: "max-w-[420px]",
  md: "max-w-[520px]",
  lg: "max-w-[760px]",
};

const FOCUSABLE_SELECTOR =
  "button:not([disabled]), [href], input, select, textarea, [tabindex]:not([tabindex='-1'])";

/**
 * Shared dialog behaviour for Modal and Drawer: focus the first focusable
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
      className="settle-scrim fixed inset-0 z-50 flex items-center justify-center bg-black/55 px-4"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      {/* The scrim fades, then the card rises 10px from 97% a beat behind it.
          It never zooms out of screen centre — the destination is where the
          dialog will live, so the eye lands there and stays. */}
      <div
        ref={panelRef}
        className={`w-full ${SIZE_CLASS[size]} settle-in overflow-hidden rounded-[22px] border border-line-strong bg-surface-2 shadow-dialog`}
      >
        {children}
      </div>
    </div>
  );
}

/** Dialog header: an optional source chip above the title, then the title. */
export function ModalHeader({
  chip,
  title,
  lede,
  titleId,
}: {
  chip?: React.ReactNode;
  title: string;
  lede?: React.ReactNode;
  titleId?: string;
}) {
  return (
    <div className="px-6 pt-[22px]">
      {chip && <div className="mb-3.5">{chip}</div>}
      <h2 id={titleId} className="type-dialog-title mb-2.5">
        {title}
      </h2>
      {lede && <p className="mb-4 text-[14.5px] leading-[1.55] text-muted">{lede}</p>}
    </div>
  );
}

export function ModalFooter({
  children,
  note,
}: {
  children: React.ReactNode;
  note?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2.5 px-6 pb-[22px] pt-5">
      <div className="flex flex-wrap items-center gap-2.5">{children}</div>
      {note && <div className="text-[12.5px] leading-[1.5] text-faint">{note}</div>}
    </div>
  );
}

"use client";

import React from "react";

import { Modal } from "@/components/ui/Modal";

interface ConfirmModalProps {
  open: boolean;
  title: string;
  /**
   * Description copy. Accepts ReactNode so callers can compose Name components
   * (e.g. <RoleName/>) inline — required for spec compliance: confirmation
   * copy MUST display resolved entity names, never raw `project_id:role_key`
   * strings.
   */
  description: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "primary" | "destructive";
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
  isPending?: boolean;
}

/**
 * Confirmation modal for destructive or otherwise irreversible actions.
 * Composes the generic <Modal/> primitive (focus trap, aria-modal, Esc +
 * click-outside dismiss) with a destructive-variant footer. Replaces native
 * window.confirm() so dialogs match the design language and are keyboard
 * navigable. The prop surface is preserved from the original implementation
 * so existing call sites need no changes.
 */
export function ConfirmModal({
  open,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "primary",
  onConfirm,
  onCancel,
  isPending = false,
}: ConfirmModalProps) {
  const confirmClasses =
    variant === "destructive"
      ? "bg-error-container text-on-error-container hover:opacity-90"
      : "bg-primary text-on-primary hover:bg-primary-container";

  return (
    <Modal
      open={open}
      onClose={onCancel}
      busy={isPending}
      labelledBy="confirm-title"
      describedBy="confirm-description"
      size="md"
    >
      <h2 id="confirm-title" className="text-lg font-semibold text-on-surface">
        {title}
      </h2>
      <p id="confirm-description" className="mt-2 text-sm text-on-surface-variant">
        {description}
      </p>
      <div className="mt-6 flex justify-end gap-3">
        <button
          type="button"
          onClick={onCancel}
          disabled={isPending}
          className="rounded-full border border-outline-variant px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:bg-surface-container-high disabled:cursor-not-allowed disabled:opacity-50"
        >
          {cancelLabel}
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={isPending}
          aria-busy={isPending || undefined}
          className={`inline-flex items-center justify-center gap-2 rounded-full px-4 py-2 text-sm font-semibold shadow-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${confirmClasses}`}
        >
          {isPending && (
            <span
              aria-hidden="true"
              className="h-4 w-4 animate-spin rounded-full border-2 border-current/40 border-t-current"
            />
          )}
          {isPending ? "Working…" : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}

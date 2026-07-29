"use client";

import { useRef } from "react";

import { useDialogFocusTrap } from "@/components/ui/Modal";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  labelledBy?: string;
  describedBy?: string;
  busy?: boolean;
  /** Drawer width — sm: 24rem, md: 32rem, lg: 40rem. Default md. */
  size?: "sm" | "md" | "lg";
  children: React.ReactNode;
}

const SIZE_CLASS: Record<NonNullable<DrawerProps["size"]>, string> = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-2xl",
};

/**
 * Right-side sheet variant of <Modal/>. Same focus-trap, aria-modal, Esc, and
 * click-outside semantics as Modal but with slide-in-from-right geometry and
 * full-height panel. Used for audit-detail payload viewing and reconciliation
 * drift drill-in.
 */
export function Drawer({
  open,
  onClose,
  labelledBy,
  describedBy,
  busy = false,
  size = "md",
  children,
}: DrawerProps) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  useDialogFocusTrap(panelRef, open, busy, onClose);

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className="fixed inset-0 z-50 flex items-stretch justify-end bg-black/50 backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      <div
        ref={panelRef}
        className={`glass-card w-full ${SIZE_CLASS[size]} h-full overflow-y-auto p-6 rounded-l-card rounded-r-none animate-fade-in-up`}
      >
        {children}
      </div>
    </div>
  );
}

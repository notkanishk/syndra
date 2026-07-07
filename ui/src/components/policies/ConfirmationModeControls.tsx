"use client";

import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { useBulkSetConfirmationMode, type ConfirmationMode } from "@/lib/queries/useConfirmationMode";
import { toastError, toastSuccess } from "@/lib/toast";

interface ConfirmationModeControlsProps {
  kind: "rule" | "bundle";
  /** Selected row ids, managed by the parent page alongside its per-row checkboxes. */
  selectedIds: Set<string>;
  /** Called after a successful bulk apply — the parent clears selection / bulk-edit mode. */
  onDone: () => void;
}

/**
 * Bulk confirmation-mode apply toolbar (Task 22). The parent page owns the
 * "Bulk edit" toggle and the per-row checkboxes that populate `selectedIds`
 * (those live on the row markup); this component is mounted only while bulk
 * edit is active and is just the mode picker + "Apply to N selected" action.
 */
export function ConfirmationModeControls({ kind, selectedIds, onDone }: ConfirmationModeControlsProps) {
  const [mode, setMode] = useState<ConfirmationMode>("auto");
  const bulkSet = useBulkSetConfirmationMode();

  async function apply() {
    if (selectedIds.size === 0) return;
    try {
      await bulkSet.mutateAsync({ kind, ids: Array.from(selectedIds), mode });
      toastSuccess(
        `Confirmation mode set to ${mode}`,
        `${selectedIds.size} ${kind}${selectedIds.size === 1 ? "" : "s"} updated.`,
      );
      onDone();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to update confirmation mode");
    }
  }

  return (
    <div className="flex items-center gap-2 flex-wrap rounded-card border border-outline-variant bg-surface-container-low px-3 py-2">
      <span className="text-xs text-on-surface-variant">{selectedIds.size} selected</span>
      <Select
        value={mode}
        onChange={(event) => setMode(event.target.value as ConfirmationMode)}
        aria-label="Confirmation mode"
        className="w-auto"
      >
        <option value="auto">Auto</option>
        <option value="manual">Manual</option>
      </Select>
      <Button
        size="sm"
        onClick={apply}
        disabled={selectedIds.size === 0}
        isPending={bulkSet.isPending}
      >
        Apply to {selectedIds.size} selected
      </Button>
    </div>
  );
}

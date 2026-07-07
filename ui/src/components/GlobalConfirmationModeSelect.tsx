"use client";

import { Select } from "@/components/ui/Select";
import {
  useGlobalConfirmationDefault,
  useSetGlobalConfirmationDefault,
  type ConfirmationMode,
} from "@/lib/queries/useConfirmationMode";
import { toastError, toastSuccess } from "@/lib/toast";

/**
 * Operator-only global default confirmation-mode control (Task 22), mounted
 * in the sidebar footer beside ChimeToggle/ThemeToggle — the same "no
 * dedicated /settings page" location the drift chime kill-switch uses (see
 * design.md §9). Every new bundle/rule inherits this default unless the
 * create form overrides it.
 */
export default function GlobalConfirmationModeSelect() {
  const query = useGlobalConfirmationDefault();
  const setDefault = useSetGlobalConfirmationDefault();

  async function onChange(mode: ConfirmationMode) {
    try {
      await setDefault.mutateAsync(mode);
      toastSuccess(`Global default confirmation mode set to ${mode}`);
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to update global default");
    }
  }

  return (
    <label className="flex items-center justify-between gap-2 text-xs text-on-surface-variant">
      <span>Default confirmation mode</span>
      <Select
        value={query.data ?? "auto"}
        onChange={(event) => onChange(event.target.value as ConfirmationMode)}
        aria-label="Global default confirmation mode"
        disabled={query.isLoading || setDefault.isPending}
        className="w-auto py-1 text-xs"
      >
        <option value="auto">Auto</option>
        <option value="manual">Manual</option>
      </Select>
    </label>
  );
}

"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/Button";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { useCreateBundle } from "@/lib/queries/useBundles";
import { useGlobalConfirmationDefault, type ConfirmationMode } from "@/lib/queries/useConfirmationMode";
import { toastError, toastSuccess } from "@/lib/toast";

interface CreateBundleModalProps {
  open: boolean;
  onClose: () => void;
  /** Optional — invoked after a successful create with the new bundle id. */
  onCreated?: (bundleId: string) => void;
}

/**
 * Modal-wrapped bundle authoring form. Stage 4 lifts the inline create form
 * out of `/bundles/page.tsx` so the create flow gets focus-trap, Esc dismiss,
 * and the same governance-first ergonomics other admin mutations rely on.
 *
 * The form is intentionally minimal — name + description — because bundles
 * are containers for roles. Roles are added via `<AddRolesToBundlePicker/>`
 * in a separate, scoped flow once the bundle exists.
 */
export default function CreateBundleModal({ open, onClose, onCreated }: CreateBundleModalProps) {
  const createBundle = useCreateBundle();
  const globalDefault = useGlobalConfirmationDefault();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  // null until the operator touches the selector — the displayed (and submitted) value falls
  // back to the fetched global default, else "auto", so the form is always a WYSIWYG submit.
  const [modeOverride, setModeOverride] = useState<ConfirmationMode | null>(null);
  const confirmationMode: ConfirmationMode = modeOverride ?? globalDefault.data ?? "auto";

  // Reset on close so the next open starts fresh — preserves the operator's
  // expectation that an aborted create doesn't leak into the next attempt.
  useEffect(() => {
    if (!open) {
      setName("");
      setDescription("");
      setModeOverride(null);
    }
  }, [open]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;
    try {
      const result = await createBundle.mutateAsync({
        name: trimmed,
        description: description.trim(),
        confirmation_mode: confirmationMode,
      });
      toastSuccess("Bundle created", `"${trimmed}" is ready to receive roles.`);
      onCreated?.(result.id);
      onClose();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to create bundle");
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      busy={createBundle.isPending}
      labelledBy="create-bundle-title"
      size="md"
    >
      <Eyebrow>New bundle</Eyebrow>
      <h2 id="create-bundle-title" className="text-lg font-semibold text-on-surface mt-1">
        Create a role bundle
      </h2>
      <p className="mt-1 text-sm text-on-surface-variant">
        Bundles group related roles into a reusable assignment. Add roles after
        creation so the audit trail captures the composition step-by-step.
      </p>

      <form onSubmit={handleSubmit} className="mt-5 space-y-4">
        <div>
          <label htmlFor="create-bundle-name" className="block text-xs font-medium text-on-surface-variant mb-1.5">
            Bundle name
          </label>
          <Input
            id="create-bundle-name"
            required
            placeholder="e.g. Workshop Mentors"
            value={name}
            onChange={(event) => setName(event.target.value)}
            autoFocus
          />
        </div>
        <div>
          <label htmlFor="create-bundle-description" className="block text-xs font-medium text-on-surface-variant mb-1.5">
            Description <span className="text-on-surface-variant/70">(optional)</span>
          </label>
          <Input
            id="create-bundle-description"
            placeholder="What does this bundle grant?"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </div>
        <div>
          <label htmlFor="create-bundle-mode" className="block text-xs font-medium text-on-surface-variant mb-1.5">
            Confirmation mode
          </label>
          <Select
            id="create-bundle-mode"
            value={confirmationMode}
            onChange={(event) => setModeOverride(event.target.value as ConfirmationMode)}
          >
            <option value="auto">Auto — drains immediately</option>
            <option value="manual">Manual — waits for operator resume</option>
          </Select>
        </div>

        <div className="flex items-center justify-end gap-3 pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <SubmitButton
            isPending={createBundle.isPending}
            disabled={name.trim().length === 0}
            pendingLabel="Creating…"
            label="Create bundle"
          />
        </div>
      </form>
    </Modal>
  );
}
